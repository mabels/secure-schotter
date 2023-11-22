package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"

	// ctd "github.com/containerd/containerd/v2"
	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	snapshotsvc "github.com/containerd/containerd/contrib/snapshotservice"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type shots struct {
	key      string
	loop     string
	secure   string
	finalDir string
	mapped   string
}

type MyCoolSnapshotter struct {
	basePath  string
	loopPaths map[string]shots
}

func NewMyCoolSnapshotter() (*MyCoolSnapshotter, error) {
	return &MyCoolSnapshotter{
		basePath:  "/var/tmp/schotter.dir",
		loopPaths: map[string]shots{},
	}, nil
}

func (s *MyCoolSnapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	fmt.Printf("Stat %s\n", key)
	return snapshots.Info{
		Kind: snapshots.KindUnknown,
	}, nil
}

func (s *MyCoolSnapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	panic("update not implemented")

}

func (s *MyCoolSnapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	panic("usage not implemented")

}

func (s *MyCoolSnapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	fmt.Printf("Mounts %s\n", key)
	prepared, ok := s.loopPaths[key]
	if !ok {
		log.Error().Str("key", key).Msg("no such key")
		return nil, fmt.Errorf("no such key %s", key)
	}
	// finalDir := path.Join(s.basePath, key)
	// err := exec.Command("/usr/bin/mount", "-o", "rw", prepared.mapped, finalDir).Run()
	// if err != nil {
	// 	log.Error().Err(err).Str("mapped", prepared.mapped).Str("finalDir", finalDir).Msg("mount")
	// 	return nil, err
	// }
	return []mount.Mount{
		{
			Source: prepared.mapped,
			// Target: prepared.finalDir,
		},
	}, nil
}

func (s *MyCoolSnapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	fmt.Printf("Prepare %s %s %v\n", key, parent, opts)
	fname := path.Join(s.basePath, key)
	err := os.MkdirAll(fname, 0755)
	if err != nil {
		log.Error().Err(err).Msg("mkdir")
		return nil, err
	}
	secureFname := fname + ".secure"
	file, err := os.Create(secureFname)
	if err != nil {
		log.Error().Err(err).Msg("create")
		return nil, err
	}
	_, err = file.Seek((1024*1024*1024)-1, 0)
	if err != nil {
		log.Error().Err(err).Msg("seek")

		return nil, err
	}
	_, err = file.Write([]byte{0})
	if err != nil {
		log.Error().Err(err).Msg("write")
		return nil, err
	}
	file.Close()

	loSetup := exec.Command("/usr/sbin/losetup", "-f", "--show", secureFname)
	byteLoopDev, err := loSetup.Output()
	if err != nil {
		log.Error().Str("secureFname", secureFname).Err(err).Msg("losetup")
		return nil, err
	}
	loopDev := strings.TrimSpace(string(byteLoopDev))
	cryptoFname := "schotter-" + strings.ReplaceAll(fname, "/", "_")
	cryptSetup := exec.Command("/usr/sbin/cryptsetup",
		"-q", "plainOpen", "--key-file", "/dev/urandom",
		loopDev, cryptoFname)
	err = cryptSetup.Run()
	if err != nil {
		log.Error().Err(err).Msg("cryptsetup")
		return nil, err
	}

	mappedPath := path.Join("/dev/mapper", cryptoFname)

	err = exec.Command("/usr/sbin/mkfs.ext4", mappedPath).Run()
	if err != nil {
		log.Error().Err(err).Msg("mkfs")
		return nil, err
	}
	finalDir := path.Join(s.basePath, key)
	s.loopPaths[key] = shots{
		key:      key,
		secure:   secureFname,
		mapped:   mappedPath,
		loop:     loopDev,
		finalDir: finalDir,
	}
	// cryptsetup -q plainOpen --key-file /dev/urandom /dev/loop52 doof
	// cryptsetup plainClose doof

	return []mount.Mount{
		{
			Source: mappedPath,
			Target: finalDir,
		},
	}, nil
}

func (s *MyCoolSnapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	panic("view not implemented")

}

func (s *MyCoolSnapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	fmt.Printf("Commit %s %s\n", name, key)
	return nil
}

func (s *MyCoolSnapshotter) Remove(ctx context.Context, key string) error {
	panic("remove not implemented")
}

func (s *MyCoolSnapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, filters ...string) error {
	fmt.Printf("Walk %v => %v\n", filters, s.loopPaths)
	for _, v := range s.loopPaths {
		fn(ctx, snapshots.Info{
			Kind:   snapshots.KindActive,
			Parent: v.finalDir,
			Name:   v.key,
		})
	}
	return nil
}

func (s *MyCoolSnapshotter) Close() error {
	fmt.Println("Close")
	return nil
}

func main() {
	// Create a gRPC server
	rpc := grpc.NewServer()

	// Configure your custom snapshotter
	sn, _ := NewMyCoolSnapshotter()
	// Convert the snapshotter to a gRPC service,
	// ctd.WithSnapshotService(sn)
	service := snapshotsvc.FromSnapshotter(sn)
	// // Register the service with the gRPC server
	snapshotsapi.RegisterSnapshotsServer(rpc, service)

	os.Remove("/tmp/schotter")
	// Listen and serve
	l, _ := net.Listen("unix", "/tmp/schotter")
	rpc.Serve(l)
}
