package remotecli

import (
	"context"
	"fmt"
	"os"
	"path"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	reflectionv1 "cosmossdk.io/api/cosmos/reflection/v1"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/ignite/cli/v29/ignite/pkg/errors"
)

type ChainInfo struct {
	client *grpc.ClientConn

	Context   context.Context
	ConfigDir string
	Chain     string
	Config    interface{}

	ProtoFiles    *protoregistry.Files
	ModuleOptions map[string]*autocliv1.ModuleOptions
}

func NewChainInfo(configDir, chain string, config interface{}) *ChainInfo {
	return &ChainInfo{
		Context:   context.Background(),
		Config:    config,
		Chain:     chain,
		ConfigDir: configDir,
	}
}

func (c *ChainInfo) getCacheDir() (string, error) {
	cacheDir := path.Join(c.ConfigDir, "cache")
	return cacheDir, os.MkdirAll(cacheDir, 0o750)
}

func (c *ChainInfo) fdsCacheFilename() (string, error) {
	cacheDir, err := c.getCacheDir()
	if err != nil {
		return "", err
	}
	return path.Join(cacheDir, fmt.Sprintf("%s.fds", c.Chain)), nil
}

func (c *ChainInfo) appOptsCacheFilename() (string, error) {
	cacheDir, err := c.getCacheDir()
	if err != nil {
		return "", err
	}
	return path.Join(cacheDir, fmt.Sprintf("%s.autocli", c.Chain)), nil
}

func (c *ChainInfo) Load(reload bool) error {
	fdSet := &descriptorpb.FileDescriptorSet{}
	fdsFilename, err := c.fdsCacheFilename()
	if err != nil {
		return err
	}

	if _, err := os.Stat(fdsFilename); os.IsNotExist(err) || reload {
		client, err := c.OpenClient()
		if err != nil {
			return err
		}

		reflectionClient := reflectionv1.NewReflectionServiceClient(client)
		fdRes, err := reflectionClient.FileDescriptors(c.Context, &reflectionv1.FileDescriptorsRequest{})
		if err != nil {
			return err
		}
		fdSet = &descriptorpb.FileDescriptorSet{File: fdRes.Files}

		bz, err := proto.Marshal(fdSet)
		if err != nil {
			return err
		}

		if err = os.WriteFile(fdsFilename, bz, 0o600); err != nil {
			return err
		}
	} else {
		bz, err := os.ReadFile(fdsFilename)
		if err != nil {
			return err
		}

		if err = proto.Unmarshal(bz, fdSet); err != nil {
			return err
		}
	}

	c.ProtoFiles, err = protodesc.FileOptions{AllowUnresolvable: true}.NewFiles(fdSet)
	if err != nil {
		return errors.Errorf("error building protoregistry.Files: %w", err)
	}

	appOptsFilename, err := c.appOptsCacheFilename()
	if err != nil {
		return err
	}

	if _, err := os.Stat(appOptsFilename); os.IsNotExist(err) || reload {
		client, err := c.OpenClient()
		if err != nil {
			return err
		}

		autocliQueryClient := autocliv1.NewQueryClient(client)
		appOptsRes, err := autocliQueryClient.AppOptions(c.Context, &autocliv1.AppOptionsRequest{})
		if err != nil {
			return err
		}

		bz, err := proto.Marshal(appOptsRes)
		if err != nil {
			return err
		}

		if err := os.WriteFile(appOptsFilename, bz, 0o600); err != nil {
			return err
		}

		c.ModuleOptions = appOptsRes.ModuleOptions
	} else {
		bz, err := os.ReadFile(appOptsFilename)
		if err != nil {
			return err
		}

		var appOptsRes autocliv1.AppOptionsResponse
		if err := proto.Unmarshal(bz, &appOptsRes); err != nil {
			return err
		}

		c.ModuleOptions = appOptsRes.ModuleOptions
	}

	return nil
}
