package cloudinit

import (
	"github.com/0xef53/kvmrun/server/cloudinit"

	pb "github.com/0xef53/kvmrun/api/services/cloudinit/v2"
)

func optsFromBuildImageRequest(req *pb.BuildImageRequest) *cloudinit.CloudInitOptions {
	return &cloudinit.CloudInitOptions{
		Platform:         req.Platform,
		Subplatform:      req.Subplatform,
		Cloudname:        req.Cloudname,
		Region:           req.Region,
		AvailabilityZone: req.AvailabilityZone,
		Hostname:         req.Hostname,
		Domain:           req.Domain,
		Timezone:         req.Timezone,
		VendorConfig:     req.VendorConfig,
		UserConfig:       req.UserConfig,
	}
}
