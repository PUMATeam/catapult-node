package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/image/copy"
	"github.com/containers/image/transports/alltransports"
	log "github.com/sirupsen/logrus"

	"github.com/containers/image/signature"
	ociTools "github.com/opencontainers/image-tools/image"
)

type storage struct {
	log *log.Logger
}

func (s *storage) pullImage(ctx context.Context, imageName string) (string, error) {
	sanitizedImageName := strings.Replace(imageName, "/", "-", -1)
	workingDir := fmt.Sprintf("/tmp/%s/", sanitizedImageName)
	tag := "tmp"
	s.log.Infof("Creating work dir")
	err := os.Mkdir(workingDir, 0755)
	if err != nil {
		s.log.Error("Failed to create directory", err)
		return "", err
	}

	policy := &signature.Policy{Default: []signature.PolicyRequirement{
		signature.NewPRInsecureAcceptAnything(),
	}}

	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		s.log.Error("Failed to policy context", err)
		return "", err
	}

	srcImageType, err :=
		alltransports.ParseImageName(fmt.Sprintf("docker://%s", imageName))
	if err != nil {
		s.log.Error("Failed to parse image", err)
		return "", err

	}

	dstImageType, err :=
		alltransports.ParseImageName(fmt.Sprintf("oci:/%s:%s", workingDir, tag))
	if err != nil {
		s.log.Error("Failed to parse image", err)
		return "", err
	}

	s.log.Info("Copying image...")
	_, err = copy.Image(ctx,
		policyCtx,
		dstImageType,
		srcImageType,
		&copy.Options{
			ReportWriter: s.log.Out,
		})

	if err != nil {
		s.log.Error("Failed to copy image")
		return "", err
	}

	s.log.Infof("Unpacking layers...")
	err = ociTools.UnpackLayout(
		workingDir,
		workingDir+"rootfs",
		"",
		[]string{fmt.Sprintf("name=%s", tag)})

	if err != nil {
		s.log.Error("Failed to unpack layers")
		return "", nil
	}

	return workingDir, nil
}

func (s *storage) createRootFS(imagePath string) (string, int64, error) {
	size, err := s.dirSize(imagePath)
	if err != nil {
		s.log.Error(err)
		return "", -1, err
	}

	s.log.Info("Creating rootfs file")
	f, err := os.Create(path.Join(imagePath, "rootfs-file"))
	if err != nil {
		s.log.Error("Failed to create rootfs file", err)
		return "", -1, err
	}

	s.log.Infof("Creating roots file %s with size %d", f.Name(), size)
	err = os.Truncate(f.Name(), size)
	if err != nil {
		s.log.Error("Failed to truncate rootfs file", err)
		return "", -1, err
	}

	s.log.Info("Creating ext4 filesystem", f.Name(), size)
	cmd := exec.Command("mkfs.ext4", "-F", path.Join(imagePath, "rootfs-file"))
	err = cmd.Run()
	if err != nil {
		s.log.Error("Failed to create filesystem", err)
		return "", -1, err
	}

	err = os.Mkdir(path.Join(imagePath, "mnt"), 0755)
	if err != nil {
		s.log.Error(err)
		return "", -1, err
	}

	s.log.Info("Mounting rootfs file", f.Name(), size)
	cmd = exec.Command("mount",
		"-o",
		"loop",
		path.Join(imagePath, "rootfs-file"),
		path.Join(imagePath, "mnt"))
	err = cmd.Run()
	if err != nil {
		s.log.Error("Failed to mount rootfs file", err)
		return "", -1, err
	}

	cmd = exec.Command("cp",
		"-r",
		path.Join(imagePath, "rootfs", "*"),
		path.Join(imagePath, "mnt"))

	cmd = exec.Command("umount", path.Join(imagePath, "mnt"))
	err = cmd.Run()
	if err != nil {
		s.log.Error("Failed to unmount rootfs file", err)
		return "", -1, err
	}

	info, err := os.Stat(path.Join(imagePath, "rootfs-file"))
	if err != nil {
		fmt.Println(err)
		return "", -1, err
	}

	return path.Join(imagePath, "rootfs-file"), info.Size(), nil
}

func (s *storage) mapVolume(volumeID, pool string) error {
	conf := "/etc/ceph/ceph.conf"
	keyring := "/etc/ceph/ceph.client.admin.keyring"

	command := []string{"map", fmt.Sprintf("%s/%s", pool, volumeID), "-k", keyring, conf}
	s.log.Infof("Executing command rbd-nbd with parameters %v", command)
	cmd := exec.Command("rbd-nbd", command...)
	err := cmd.Run()
	// TODO: check the drive actually exists

	return err
}

func (s *storage) dirSize(dir string) (int64, error) {
	var size int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
