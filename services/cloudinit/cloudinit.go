package cloudinit

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/cloudinit/v2"
)

func (s *service) BuildImage(ctx context.Context, req *pb.BuildImageRequest) (*pb.BuildImageResponse, error) {
	opts := optsFromBuildImageRequest(req)

	outfile, err := s.ServiceServer.CloudInit.BuildImage(ctx, req.MachineName, opts, req.OutputFile)
	if err != nil {
		return nil, err
	}

	return &pb.BuildImageResponse{OutputFile: outfile}, nil
}
