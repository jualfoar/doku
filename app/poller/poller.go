package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"

	"github.com/amerkurev/doku/app/docker"
	"github.com/amerkurev/doku/app/store"
	"github.com/amerkurev/doku/app/types"
	"github.com/amerkurev/doku/app/util"
)

const pollingInterval = time.Minute

// Run starts a goroutine to poll the Docker daemon.
func Run(ctx context.Context, d *docker.Client, volumes []types.HostVolume) {
	messages, errs := d.Events(ctx, dockerTypes.EventsOptions{})
	numMessages := 0 // count of Docker daemon events.

	// calculate the size of directories that mounted into containers (bind type)
	mountsBindSize(ctx, d, volumes)

	go func() {
		// run it immediately on start
		poll(ctx, d, volumes)
		lastPoll := time.Now()

		// run poll with interval while context is not cancel
		for {
			select {
			case m := <-messages:
				if docker.IsSignificantEvent(m.Type) {
					numMessages++
				}
			case err := <-errs:
				if err != nil {
					log.WithField("err", err).Error("failed to listen to docker events")

					// reconnect to the Docker daemon
					select {
					case <-time.After(pollingInterval):
						messages, errs = d.Events(ctx, dockerTypes.EventsOptions{})
					case <-ctx.Done():
						log.Info("gracefully poller shutdown")
						return
					}
				}
			case <-ctx.Done():
				log.Info("gracefully poller shutdown")
				return
			case <-time.After(time.Second):
				// execute poll only if was happened Docker daemon events
				if numMessages > 0 {
					numMessages = 0
					poll(ctx, d, volumes)
					lastPoll = time.Now()
				}

				// forced poll in a minute after the last poll
				if time.Since(lastPoll) > pollingInterval {
					poll(ctx, d, volumes)
					lastPoll = time.Now()
				}
			}
		}
	}()
}

func poll(ctx context.Context, d *docker.Client, volumes []types.HostVolume) {
	defer util.PrintExecTime("poll execution progress")()
	defer store.NotifyAll() // wake up those who are waiting.

	if err := dockerVersion(ctx, d); err != nil {
		log.WithField("err", err).Error("failed to get information of the docker client and server host")
	}

	if err := dockerDiskUsage(ctx, d); err != nil {
		log.WithField("err", err).Error("failed to request the current data usage from the docker daemon")
	}

	if err := dockerLogInfo(ctx, d, volumes); err != nil {
		log.WithField("err", err).Error("failed to get information about the container log")
	}

	fmt.Println("tut")
}

func dockerVersion(ctx context.Context, d *docker.Client) error {
	res, err := d.ServerVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}

	b, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("failed to encode as JSON: %w", err)
	}

	store.Set("dockerVersion", b)
	return nil
}

func dockerDiskUsage(ctx context.Context, d *docker.Client) error {
	res, err := d.DiskUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}

	b, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("failed to encode as JSON: %w", err)
	}

	store.Set("dockerDiskUsage", b)
	return nil
}

func dockerLogInfo(ctx context.Context, d *docker.Client, volumes []types.HostVolume) error {
	containers, err := d.ContainerJSONList(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}

	logs := make(map[string]*types.LogFileInfo, len(containers))

	for _, cont := range containers {
		l, errSize := logFileSize(cont, volumes) // get size of container log file
		if errSize != nil {
			return fmt.Errorf("failed to get log file size: %w", errSize)
		}
		logs[cont.ID] = l
	}

	b, err := json.Marshal(logs)
	if err != nil {
		return fmt.Errorf("failed to encode as JSON: %w", err)
	}

	store.Set("dockerLogInfo", b)
	return nil
}

func logFileSize(ci *dockerTypes.ContainerJSON, volumes []types.HostVolume) (*types.LogFileInfo, error) {
	var err error
	for _, vol := range volumes {
		p := path.Join(vol.Path, ci.LogPath)

		fi, statErr := os.Stat(p)
		if statErr != nil {
			err = statErr
			continue
		}

		return &types.LogFileInfo{
			ContainerID:   ci.ID,
			ContainerName: ci.Name,
			Path:          ci.LogPath,
			Size:          fi.Size(),
			LastCheck:     time.Now().UnixMilli(),
		}, nil
	}
	return nil, err // return last os.Stat error
}
