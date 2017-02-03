package kube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcloud/fissile/model"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/1.5/pkg/api/resource"
	"k8s.io/client-go/1.5/pkg/api/v1"
)

func podTestLoadRole(assert *assert.Assertions) *model.Role {
	workDir, err := os.Getwd()
	if !assert.NoError(err) {
		return nil
	}

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/volumes.yml")
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")
	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	if !assert.NoError(err) {
		return nil
	}
	manifest, err := model.LoadRoleManifest(manifestPath, []*model.Release{release})
	if !assert.NoError(err) {
		return nil
	}

	var role *model.Role
	for _, r := range manifest.Roles {
		if r != nil {
			if r.Name == "myrole" {
				role = r
			}
		}
	}
	if !assert.NotNil(role, "Failed to find role in manifest") {
		return nil
	}

	return role
}

func TestPodGetVolumes(t *testing.T) {
	assert := assert.New(t)
	role := podTestLoadRole(assert)
	if role == nil {
		return
	}

	volumes, claims := getVolumes(role)
	assert.Len(volumes, 2, "expected two volumes")
	assert.Len(claims, 2, "expected two claims")

	var persistentVolume, sharedVolume *v1.Volume
	for _, volRef := range volumes {
		vol := volRef // Do a copy so they actually refer to different volumes
		switch vol.Name {
		case "persistent-volume":
			assert.Nil(persistentVolume, "Multiple persistent volumes")
			persistentVolume = &vol
		case "shared-volume":
			assert.Nil(sharedVolume, "Multiple shared volumes")
			sharedVolume = &vol
		default:
			assert.Fail("Got unexpected volume", "%+v", vol)
		}
	}
	assert.NotNil(persistentVolume)
	assert.NotNil(sharedVolume)
	assert.NotEqual(persistentVolume, sharedVolume)

	// The persistent volume should have a related volume claim
	var persistentClaim, sharedClaim *v1.PersistentVolumeClaim
	for _, claim := range claims {
		switch claim.GetName() {
		case persistentVolume.PersistentVolumeClaim.ClaimName:
			persistentClaim = claim
		case sharedVolume.PersistentVolumeClaim.ClaimName:
			sharedClaim = claim
		default:
			assert.Fail("Got unexpected claim", "%v", claim)
		}
	}
	if assert.NotNil(persistentClaim) {
		assert.Contains(persistentClaim.Annotations, VolumeStorageClassAnnotation)
		assert.Equal("persistent", persistentClaim.Annotations[VolumeStorageClassAnnotation])
		assert.Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, persistentClaim.Spec.AccessModes)
		if assert.NotNil(persistentClaim.Spec.Resources.Requests) {
			requests := persistentClaim.Spec.Resources.Requests
			if assert.Contains(requests, v1.ResourceStorage) {
				quantity := requests[v1.ResourceStorage]
				assert.Zero(resource.NewScaledQuantity(5, resource.Giga).Cmp(quantity),
					"Storage request %s should be 5 Gigs", quantity.String())
			}
		}
	}

	for _, claim := range claims {
		if claim.GetName() == sharedVolume.PersistentVolumeClaim.ClaimName {
			sharedClaim = claim
		}
	}
	if assert.NotNil(sharedClaim) {
		assert.Contains(sharedClaim.Annotations, VolumeStorageClassAnnotation)
		assert.Equal("shared", sharedClaim.Annotations[VolumeStorageClassAnnotation])
		assert.Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteMany}, sharedClaim.Spec.AccessModes)
		if assert.NotNil(sharedClaim.Spec.Resources.Requests) {
			requests := sharedClaim.Spec.Resources.Requests
			if assert.Contains(requests, v1.ResourceStorage) {
				quantity := requests[v1.ResourceStorage]
				assert.Zero(resource.NewScaledQuantity(40, resource.Giga).Cmp(quantity),
					"Storage request %s should be 40 Gigs", quantity.String())
			}
		}
	}
}

func TestPodGetVolumeMounts(t *testing.T) {
	assert := assert.New(t)
	role := podTestLoadRole(assert)
	if role == nil {
		return
	}

	volumeMounts := getVolumeMounts(role)
	assert.Len(volumeMounts, 2)

	var persistentMount, sharedMount v1.VolumeMount
	for _, mount := range volumeMounts {
		switch mount.Name {
		case "persistent-volume":
			persistentMount = mount
		case "shared-volume":
			sharedMount = mount
		default:
			assert.Fail("Got unexpected volume mount", "%+v", mount)
		}
	}
	assert.Equal("persistent-volume", persistentMount.Name)
	assert.Equal("/mnt/persistent", persistentMount.MountPath)
	assert.False(persistentMount.ReadOnly)
	assert.Equal("shared-volume", sharedMount.Name)
	assert.Equal("/mnt/shared", sharedMount.MountPath)
	assert.False(sharedMount.ReadOnly)
}
