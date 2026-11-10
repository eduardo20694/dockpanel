package main

import (
	"encoding/json"

	dockerVolume "github.com/docker/docker/api/types/volume"
)

func dockerVolumeList() dockerVolume.ListOptions {
	return dockerVolume.ListOptions{}
}

func jsonUnmarshal(raw []byte, dest any) error {
	return json.Unmarshal(raw, dest)
}
