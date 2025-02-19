package compose

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/src-d/sourced-ce/cmd/sourced/compose/workdir"
	"github.com/src-d/sourced-ce/cmd/sourced/dir"

	"github.com/pkg/errors"
	goerrors "gopkg.in/src-d/go-errors.v1"
)

// dockerComposeVersion is the version of docker-compose to download
// if docker-compose isn't already present in the system
const dockerComposeVersion = "1.24.0"

var composeContainerURL = fmt.Sprintf("https://github.com/docker/compose/releases/download/%s/run.sh", dockerComposeVersion)

// ErrComposeAlternative is returned when docker-compose alternative could not be installed
var ErrComposeAlternative = goerrors.NewKind("error while trying docker-compose container alternative")

type Compose struct {
	bin            string
	workdirHandler *workdir.Handler
}

func (c *Compose) Run(ctx context.Context, arg ...string) error {
	return c.RunWithIO(ctx, os.Stdin, os.Stdout, os.Stderr, arg...)
}

func (c *Compose) RunWithIO(ctx context.Context, stdin io.Reader,
	stdout, stderr io.Writer, arg ...string) error {
	arg = append([]string{"--compatibility"}, arg...)
	cmd := exec.CommandContext(ctx, c.bin, arg...)

	wd, err := c.workdirHandler.Active()
	if err != nil {
		return err
	}

	if err := c.workdirHandler.Validate(wd); err != nil {
		return err
	}

	cmd.Dir = wd.Path
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

func newCompose() (*Compose, error) {
	workdirHandler, err := workdir.NewHandler()
	if err != nil {
		return nil, err
	}

	bin, err := getOrInstallComposeBinary()
	if err != nil {
		return nil, err
	}

	return &Compose{
		bin:            bin,
		workdirHandler: workdirHandler,
	}, nil
}

func getOrInstallComposeBinary() (string, error) {
	path, err := exec.LookPath("docker-compose")
	if err == nil {
		bin := strings.TrimSpace(path)
		if bin != "" {
			return bin, nil
		}
	}

	path, err = getOrInstallComposeContainer()
	if err != nil {
		return "", ErrComposeAlternative.Wrap(err)
	}

	return path, nil
}

func getOrInstallComposeContainer() (altPath string, err error) {
	datadir, err := dir.Path()
	if err != nil {
		return "", err
	}

	dirPath := filepath.Join(datadir, "bin")
	path := filepath.Join(dirPath, fmt.Sprintf("docker-compose-%s.sh", dockerComposeVersion))

	readExecAccessMode := os.FileMode(0500)

	if info, err := os.Stat(path); err == nil {
		if info.Mode()&readExecAccessMode != readExecAccessMode {
			return "", fmt.Errorf("%s can not be run", path)
		}

		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if err := downloadCompose(path); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(context.Background(), "chmod", "+x", path)
	if err := cmd.Run(); err != nil {
		return "", errors.Wrapf(err, "cannot change permission to %s", path)
	}

	return path, nil
}

func downloadCompose(path string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("compose in container is not compatible with Windows")
	}

	return dir.DownloadURL(composeContainerURL, path)
}

func Run(ctx context.Context, arg ...string) error {
	comp, err := newCompose()
	if err != nil {
		return err
	}

	return comp.Run(ctx, arg...)
}

func RunWithIO(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, arg ...string) error {
	comp, err := newCompose()
	if err != nil {
		return err
	}

	return comp.RunWithIO(ctx, stdin, stdout, stderr, arg...)
}
