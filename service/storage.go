package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/PUMATeam/catapult-node/util"
	"github.com/containers/image/copy"
	"github.com/containers/image/transports/alltransports"
	log "github.com/sirupsen/logrus"

	"github.com/containers/image/signature"
	ociTools "github.com/opencontainers/image-tools/image"
)

const (
	tag = "tmp"
)

type storage struct {
	log *log.Logger
}

// TODO create a temporary volume on the storage to handle unpacking
func (s *storage) pullImage(ctx context.Context, imageName string) (string, error) {
	sanitizedImageName := strings.Replace(imageName, "/", "-", -1)
	workingDir := fmt.Sprintf("/var/%s/", sanitizedImageName)
	s.log.Infof("Creating work dir %s", workingDir)
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

	s.log.Infof("Copying image %s...", imageName)
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

	return workingDir, nil
}

func (s *storage) unpackImage(workingDir, targetDir string) error {
	s.log.WithFields(
		log.Fields{
			"workingDir": workingDir,
			"targetDir":  targetDir}).
		Info("Unpacking layers...")
	err := ociTools.UnpackLayout(
		workingDir,
		targetDir,
		"",
		[]string{fmt.Sprintf("name=%s", tag)})

	if err != nil {
		s.log.Error("Failed to unpack layers")
		return err
	}

	return nil
}

func (s *storage) mapVolume(volumeID, pool, imagePath string) (string, error) {
	command := []string{"map", fmt.Sprintf("%s/volume-%s", pool, volumeID)}
	s.log.Infof("Executing command rbd-nbd with parameters %v", command)
	out, err := util.ExecuteCommand("rbd-nbd", command...)
	if err != nil {
		s.log.Errorf("rbd-nbd failed %s", err)
		return "", err
	}

	s.log.Infof("Creating ext4 filesystem on %s", out)
	cmd := exec.Command("mkfs.ext4", "-F", out)
	err = cmd.Run()
	if err != nil {
		s.log.Error("Failed to create filesystem: ", err)
		return "", err
	}

	mountDir := path.Join("/tmp", volumeID)
	s.log.Infof("Mounting %s on %s", out, mountDir)
	err = os.Mkdir(mountDir, 0755)
	cmd = exec.Command("mount", out, mountDir)
	err = cmd.Run()
	if err != nil {
		s.log.Errorf("Failed to mount %s", mountDir)
		return "", err
	}

	err = os.Remove(path.Join(mountDir, "lost+found"))
	if err != nil {
		s.log.Error("Failed to remove lost+found: ", err)
		return "", err
	}

	err = s.unpackImage(imagePath, mountDir)
	if err != nil {
		s.log.Error("Failed to unpack image: ", err)
		return "", err
	}

	s.log.Infof("Unmounting %s", mountDir)
	cmd = exec.Command("umount", out)
	err = cmd.Run()
	if err != nil {
		s.log.Errorf("Failed to umount %s", mountDir)
		return "", err
	}
	// TODO: check the drive actually exists

	return out, nil
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
