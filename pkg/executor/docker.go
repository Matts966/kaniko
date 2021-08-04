package executor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"sync"

	"github.com/GoogleContainerTools/kaniko/pkg/creds"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sirupsen/logrus"
)

var (
	docker *dockerService
)

func init() {
	cli, err := client.NewEnvClient()
	if err != nil {
		logrus.Error(err)
	}
	docker = &dockerService{
		Client: cli,
	}
}

type dockerService struct {
	sync.WaitGroup
	*client.Client
}

func (d *dockerService) pull(image string) {
	logrus.Info("Pulling image, ", image)
	d.Add(1)
	go func() {
		defer d.Done()
		if docker == nil {
			return
		}
		ctx := context.Background()
		ref, err := name.ParseReference(image)
		if err != nil {
			logrus.Error(err)
			return
		}
		kc, err := creds.GetKeychain().Resolve(ref.Context().Registry)
		if err != nil {
			logrus.Error(err)
			return
		}
		ac, err := kc.Authorization()
		if err != nil {
			logrus.Error(err)
			return
		}
		authConfig := types.AuthConfig{
			Username: ac.Username,
			Password: ac.Password,
		}
		encodedJSON, err := json.Marshal(authConfig)
		if err != nil {
			logrus.Error(err)
			return
		}
		rc, err := d.ImagePull(ctx, image, types.ImagePullOptions{
			RegistryAuth: base64.URLEncoding.EncodeToString(encodedJSON),
		})
		if err != nil {
			logrus.Error(err)
			return
		}
		defer rc.Close()
		d := json.NewDecoder(rc)
		for {
			var m jsonmessage.JSONMessage
			if err := d.Decode(&m); err == io.EOF {
				break
			}
			if err != nil {
				logrus.Error(err)
				return
			}
			if m.Progress == nil || m.Progress.Current == 0 {
				if m.ID != "" {
					logrus.Info(m.ID, ": ", m.Status)
					continue
				}
				logrus.Info(m.Status)
			}
		}
		logrus.Info("Pulled image, ", image)
	}()
}
