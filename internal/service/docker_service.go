package service

import (
	"context"
	"strings"
	"tinyauth/internal/config"
	"tinyauth/internal/utils/decoders"

	container "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
)

type DockerService struct {
	client      *client.Client
	context     context.Context
	isConnected bool
}

func NewDockerService() *DockerService {
	return &DockerService{}
}

func (docker *DockerService) Init() error {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}

	ctx := context.Background()
	client.NegotiateAPIVersion(ctx)

	docker.client = client
	docker.context = ctx

	_, err = docker.client.Ping(docker.context)

	if err != nil {
		log.Debug().Err(err).Msg("Docker not connected")
		docker.isConnected = false
		docker.client = nil
		docker.context = nil
		return nil
	}

	docker.isConnected = true
	log.Debug().Msg("Docker connected")

	return nil
}

func (docker *DockerService) getContainers() ([]container.Summary, error) {
	containers, err := docker.client.ContainerList(docker.context, container.ListOptions{})
	if err != nil {
		return nil, err
	}
	return containers, nil
}

func (docker *DockerService) inspectContainer(containerId string) (container.InspectResponse, error) {
	inspect, err := docker.client.ContainerInspect(docker.context, containerId)
	if err != nil {
		return container.InspectResponse{}, err
	}
	return inspect, nil
}

func (docker *DockerService) GetLabels(appDomain string) (config.App, error) {
	if !docker.isConnected {
		log.Debug().Msg("Docker not connected, returning empty labels")
		return config.App{}, nil
	}

	containers, err := docker.getContainers()
	if err != nil {
		return config.App{}, err
	}

	for _, ctr := range containers {
		inspect, err := docker.inspectContainer(ctr.ID)
		if err != nil {
			return config.App{}, err
		}

		labels, err := decoders.DecodeLabels(inspect.Config.Labels)
		if err != nil {
			return config.App{}, err
		}

		for appName, appLabels := range labels.Apps {
			if appLabels.Config.Domain == appDomain {
				log.Debug().Str("id", inspect.ID).Str("name", inspect.Name).Msg("Found matching container by domain")
				return appLabels, nil
			}

			if strings.TrimPrefix(inspect.Name, "/") == appName {
				log.Debug().Str("id", inspect.ID).Str("name", inspect.Name).Msg("Found matching container by app name")
				return appLabels, nil
			}
		}
	}

	log.Debug().Msg("No matching container found, returning empty labels")
	return config.App{}, nil
}
