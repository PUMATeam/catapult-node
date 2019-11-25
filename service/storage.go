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

func pullImage(ctx context.Context, imageName string) (string, error) {
	sanitizedImageName := strings.Replace(imageName, "/", "-", -1)
	workingDir := fmt.Sprintf("/tmp/%s/", sanitizedImageName)
	tag := "tmp"
	log.
		WithContext(ctx).
		WithField("dir", workingDir).
		Info("Creating work dir")

	err := os.Mkdir(workingDir, 0755)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	policy := &signature.Policy{Default: []signature.PolicyRequirement{
		signature.NewPRInsecureAcceptAnything(),
	}}

	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	srcImageType, err :=
		alltransports.ParseImageName(fmt.Sprintf("docker://%s", imageName))
	if err != nil {
		fmt.Println(err)
		return "", err

	}

	dstImageType, err :=
		alltransports.ParseImageName(fmt.Sprintf("oci:/%s:%s", workingDir, tag))
	if err != nil {
		return "", err
	}

	fmt.Println("Copy")
	_, err = copy.Image(ctx,
		policyCtx,
		dstImageType,
		srcImageType,
		&copy.Options{
			ReportWriter: os.Stdout,
		})

	if err != nil {
		return "", err
	}

	fmt.Println("UnpackLayout")
	err = ociTools.UnpackLayout(
		workingDir,
		workingDir+"rootfs",
		"",
		[]string{fmt.Sprintf("name=%s", tag)})

	if err != nil {
		fmt.Println(err)
		return "", nil
	}

	return workingDir, nil
}

func createRootFS(imagePath string) (string, int64, error) {
	size, err := dirSize(imagePath)
	if err != nil {
		fmt.Println(err)
		return "", -1, err
	}

	fmt.Println("Create file")

	f, err := os.Create(path.Join(imagePath, "rootfs-file"))
	if err != nil {
		fmt.Println(err)
		return "", -1, err
	}

	fmt.Println("Truncate")
	err = os.Truncate(f.Name(), size)
	if err != nil {
		fmt.Println(err)
		return "", -1, err
	}

	fmt.Println("mkfs")
	cmd := exec.Command("mkfs.ext4", "-F", path.Join(imagePath, "rootfs-file"))
	err = cmd.Run()
	if err != nil {
		fmt.Println(err)
		return "", -1, err
	}

	fmt.Println("mount")
	err = os.Mkdir(path.Join(imagePath, "mnt"), 0755)
	if err != nil {
		fmt.Println(err)
		return "", -1, err
	}

	cmd = exec.Command("mount",
		"-o",
		"loop",
		path.Join(imagePath, "rootfs-file"),
		path.Join(imagePath, "mnt"))
	err = cmd.Run()
	if err != nil {
		fmt.Println(err)
		return "", -1, err
	}

	fmt.Println("cp")
	cmd = exec.Command("cp",
		"-r",
		path.Join(imagePath, "rootfs", "*"),
		path.Join(imagePath, "mnt"))

	fmt.Println("umount")
	cmd = exec.Command("umount", path.Join(imagePath, "mnt"))
	err = cmd.Run()
	if err != nil {
		fmt.Println(err)
		return "", -1, err
	}

	fmt.Println("stat")
	info, err := os.Stat(path.Join(imagePath, "rootfs-file"))
	if err != nil {
		fmt.Println(err)
		return "", -1, err
	}

	return path.Join(imagePath, "rootfs-file"), info.Size(), nil
}

func mapVolume(volumeID, pool string) error {
	fmt.Println("Stat")
	conf := "/etc/ceph/ceph.conf"
	keyring := "/etc/ceph/ceph.client.admin.keyring"

	cmd := exec.Command("rbd", "map", fmt.Sprintf("%s/%s", pool, volumeID), "-k", keyring, conf)
	err := cmd.Run()
	// TODO: check the drive actually exists

	return err
}

func dirSize(dir string) (int64, error) {
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
