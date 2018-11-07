package compilator

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/fissile/docker"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/scripts/compilation"
	"github.com/fatih/color"
)

func (c *Compilator) compilePackageInMountNS(pkg *model.Package) (err error) {
	// Prepare input dir (package plus deps)
	if err := c.createCompilationDirStructure(pkg); err != nil {
		return fmt.Errorf("failed to create directory: %s", err)
	}

	if err := c.copyDependencies(pkg); err != nil {
		return fmt.Errorf("failed to copy dependencies: %s", err)
	}

	// Generate a compilation script
	targetScriptName := "compile.sh"
	hostScriptPath := filepath.Join(pkg.GetTargetPackageSourcesDir(c.hostWorkDir), targetScriptName)
	if err := compilation.SaveScript(c.baseType, compilation.CompilationScript, hostScriptPath); err != nil {
		return fmt.Errorf("failed to copy compilation script: %s", err)
	}

	// Extract package
	extractDir := c.getSourcePackageDir(pkg)
	if _, err := pkg.Extract(extractDir); err != nil {
		return fmt.Errorf("failed to extract package: %s", err)
	}

	// in-memory buffer of the log
	log := new(bytes.Buffer)

	stdoutWriter := docker.NewFormattingWriter(
		log,
		func(line string) string {
			return color.GreenString("compilation-%s > %s", color.MagentaString("%s", pkg.Name), color.WhiteString("%s", line))
		},
	)
	stderrWriter := docker.NewFormattingWriter(
		log,
		func(line string) string {
			return color.GreenString("compilation-%s > %s", color.MagentaString("%s", pkg.Name), color.RedString("%s", line))
		},
	)

	bashPath, err := exec.LookPath("bash")
	if err != nil {
		return fmt.Errorf("Failed to find bash: %s", err)
	}
	cmd := &exec.Cmd{
		Path:   bashPath,
		Args:   []string{"bash", hostScriptPath, pkg.Name, pkg.Version, c.hostWorkDir},
		Env:    append(os.Environ(), "HOST_USERID=1000", "HOST_USERGID=1000"),
		Dir:    c.hostWorkDir,
		Stdout: stdoutWriter,
		Stderr: stderrWriter,
		SysProcAttr: &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWNS,
		},
	}
	err = cmd.Run()
	if err != nil {
		log.WriteTo(c.ui)
		if exitError, ok := err.(*exec.ExitError); ok {
			if waitStatus, ok := exitError.Sys().(*syscall.WaitStatus); ok {
				return fmt.Errorf("Error - compilation for package %s exited with code %d", pkg.Name, waitStatus.ExitStatus())
			}
		}
		return fmt.Errorf("Error compiling package %s: %s", pkg.Name, err)
	}

	return os.Rename(
		pkg.GetPackageCompiledTempDir(c.hostWorkDir),
		pkg.GetPackageCompiledDir(c.hostWorkDir))
}
