// +build !linux

package compilator

import (
	"fmt"

	"code.cloudfoundry.org/fissile/model"
)

func (c *Compilator) compilePackageInMountNS(pkg *model.Package) (err error) {
	return fmt.Errorf("Compilation without docker is not supported outside Linux")
}
