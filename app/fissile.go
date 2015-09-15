package app

import ()

type Fissile interface {
	ListPackages()
	ListJobs()
	ListFullConfiguration()
}

type FissileApp struct {
}

func NewFissileApp() Fissile {
	return &FissileApp{}
}

func (f *FissileApp) ListPackages() {

}

func (f *FissileApp) ListJobs() {

}
func (f *FissileApp) ListFullConfiguration() {

}
