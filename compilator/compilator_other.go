// +build !linux

package compilator

import (
	"fmt"

	"github.com/hpcloud/fissile/model"
)

func (c *Compilator) compilePackageInChroot(pkg *model.Package) (err error) {
	return fmt.Errorf("Compilation without docker is not supports outside Linux")
}
