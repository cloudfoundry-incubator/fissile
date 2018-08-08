package kube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SUSE/fissile/helm"
	"github.com/SUSE/fissile/model"
	"github.com/SUSE/fissile/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serviceTestLoadRole(assert *assert.Assertions, manifestName string) (*model.RoleManifest, *model.InstanceGroup) {
	workDir, err := os.Getwd()
	assert.NoError(err)

	manifestPath := filepath.Join(workDir, "../test-assets/role-manifests/kube", manifestName)
	releasePath := filepath.Join(workDir, "../test-assets/tor-boshrelease")
	releasePathBoshCache := filepath.Join(releasePath, "bosh-cache")

	release, err := model.NewDevRelease(releasePath, "", "", releasePathBoshCache)
	if !assert.NoError(err) {
		return nil, nil
	}
	manifest, err := model.LoadRoleManifest(manifestPath, []*model.Release{release}, nil)
	if !assert.NoError(err) {
		return nil, nil
	}
	role := manifest.LookupInstanceGroup("myrole")
	if !assert.NotNil(role, "Failed to find role in manifest") {
		return nil, nil
	}
	return manifest, role
}

func TestServiceKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.JobReferences[0].ContainerProperties.BoshContainerization.Ports[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := newService(role, role.JobReferences[0], newServiceTypePrivate, ExportSettings{})
	require.NoError(t, err)
	require.NotNil(t, service)

	actual, err := RoundtripKube(service)
	require.NoError(t, err)
	testhelpers.IsYAMLSubsetString(assert, `---
		metadata:
			name: myrole-tor
		spec:
			ports:
			-
				name: http
				port: 80
				targetPort: 8080
			-
				name: https
				port: 443
				targetPort: 443
			selector:
				skiff-role-name: myrole
	`, actual)
}

func TestServiceHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.JobReferences[0].ContainerProperties.BoshContainerization.Ports[0]
	require.NotNil(t, portDef)
	service, err := newService(role, role.JobReferences[0], newServiceTypePrivate, ExportSettings{
		CreateHelmChart: true,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	t.Run("ClusterIP", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": nil,
		}
		actual, err := RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-tor"
			spec:
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: 8080
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: 443
				selector:
					skiff-role-name: "myrole"
		`, actual)
	})

	t.Run("LoadBalancer", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": "true",
		}

		actual, err := RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-tor"
			spec:
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: 8080
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: 443
				selector:
					skiff-role-name: "myrole"
		`, actual)
	})
}

func TestHeadlessServiceKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.JobReferences[0].ContainerProperties.BoshContainerization.Ports[0]
	require.NotNil(t, portDef)

	service, err := newService(role, role.JobReferences[0], newServiceTypeHeadless, ExportSettings{})
	require.NoError(t, err)
	require.NotNil(t, service)

	actual, err := RoundtripKube(service)
	require.NoError(t, err)
	testhelpers.IsYAMLSubsetString(assert, `---
		metadata:
			name: myrole-tor-set
		spec:
			ports:
			-
				name: http
				port: 80
				# targetPort must be undefined for headless services
				targetPort: 0
			-
				name: https
				port: 443
				# targetPort must be undefined for headless services
				targetPort: 0
			selector:
				skiff-role-name: myrole
			clusterIP: None
	`, actual)
}

func TestHeadlessServiceHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.JobReferences[0].ContainerProperties.BoshContainerization.Ports[0]
	require.NotNil(t, portDef)

	service, err := newService(role, role.JobReferences[0], newServiceTypeHeadless, ExportSettings{
		CreateHelmChart: true,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	t.Run("ClusterIP", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": nil,
		}
		actual, err := RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-tor-set"
			spec:
				clusterIP: "None"
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: 0
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: 0
				selector:
					skiff-role-name: "myrole"
		`, actual)
	})

	t.Run("LoadBalancer", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": "true",
		}

		actual, err := RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-tor-set"
			spec:
				clusterIP: "None"
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: 0
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: 0
				selector:
					skiff-role-name: "myrole"
		`, actual)
	})
}

func TestPublicServiceKube(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.JobReferences[0].ContainerProperties.BoshContainerization.Ports[0]
	require.NotNil(t, portDef)

	service, err := newService(role, role.JobReferences[0], newServiceTypePublic, ExportSettings{})
	require.NoError(t, err)
	require.NotNil(t, service)

	actual, err := RoundtripKube(service)
	require.NoError(t, err)
	testhelpers.IsYAMLSubsetString(assert, `---
		metadata:
			name: myrole-tor-public
		spec:
			externalIPs: [ 192.168.77.77 ]
			ports:
			-
				name: https
				port: 443
				targetPort: 443
			selector:
				skiff-role-name: myrole
	`, actual)
}

func TestPublicServiceHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.JobReferences[0].ContainerProperties.BoshContainerization.Ports[0]
	require.NotNil(t, portDef)

	service, err := newService(role, role.JobReferences[0], newServiceTypePublic, ExportSettings{
		CreateHelmChart: true,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	t.Run("ClusterIP", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.kube.external_ips":     "[127.0.0.1,127.0.0.2]",
			"Values.services.loadbalanced": nil,
		}

		actual, err := RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-tor-public"
			spec:
				externalIPs: "[127.0.0.1,127.0.0.2]"
				ports:
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: 443
				selector:
					skiff-role-name: "myrole"
		`, actual)
	})

	t.Run("LoadBalancer", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": "true",
			"Values.kube.external_ips":     "[127.0.0.1,127.0.0.2]",
		}

		actual, err := RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-tor-public"
			spec:
				ports:
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: 443
				selector:
					skiff-role-name: "myrole"
				type:	LoadBalancer
		`, actual)
	})
}

func TestActivePassiveService(t *testing.T) {
	t.Parallel()
	manifest, role := serviceTestLoadRole(assert.New(t), "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	require.NotNil(t, role.Run, "Role has no run information")
	require.NotEmpty(t, role.JobReferences[0].ContainerProperties.BoshContainerization.Ports, "Role has no exposed ports")
	role.Tags = []model.RoleTag{model.RoleTagActivePassive}

	const (
		withKube             = "kube"
		withHelm             = "helm"
		withHelmLoadBalancer = "helm with load balancer"
	)
	const (
		withClustering    = "clustering"
		withOutClustering = "not clustering"
	)

	for _, variant := range []string{withKube, withHelm, withHelmLoadBalancer} {
		func(variant string) {
			t.Run(variant, func(t *testing.T) {
				t.Parallel()
				roundTrip := func(node helm.Node) (interface{}, error) {
					switch variant {
					case withKube:
						return RoundtripKube(node)
					case withHelm:
						config := map[string]interface{}{
							"Values.kube.external_ips": []string{"192.0.2.42"},
						}
						return RoundtripNode(node, config)
					case withHelmLoadBalancer:
						config := map[string]interface{}{
							"Values.kube.external_ips":     []string{"192.0.2.42"},
							"Values.services.loadbalanced": "true",
						}
						return RoundtripNode(node, config)
					}
					panic("Unexpected variant " + variant)
				}

				for _, clustering := range []string{withClustering, withOutClustering} {
					func(clustering string) {
						t.Run(clustering, func(t *testing.T) {
							t.Parallel()

							exportSettings := ExportSettings{CreateHelmChart: variant != withKube}
							services, err := NewServiceList(role, clustering == withClustering, exportSettings)
							require.NoError(t, err)
							require.NotNil(t, services, "No services created")

							var headlessService, privateService, publicService helm.Node
							for _, service := range services.Get("items").Values() {
								serviceName := service.Get("metadata", "name").String()
								switch serviceName {
								case "myrole-tor-set":
									headlessService = service
								case "myrole-tor":
									privateService = service
								case "myrole-tor-public":
									publicService = service
								default:
									assert.Fail(t, "Unexpected service "+serviceName)
								}
							}

							if clustering == withClustering {
								if assert.NotNil(t, headlessService, "headless service not found") {
									actual, err := roundTrip(headlessService)
									if assert.NoError(t, err) {
										expected := `---
											apiVersion: v1
											kind: Service
											metadata:
												name: myrole-tor-set
											spec:
												clusterIP: None
												ports:
												-
													name: http
													port: 80
													protocol: TCP
													targetPort: 0
												-
													name: https
													port: 443
													protocol: TCP
													targetPort: 0
												selector:
													skiff-role-name: myrole
													skiff-role-active: "true"
										`
										testhelpers.IsYAMLEqualString(assert.New(t), expected, actual)
									}
								}
							} else {
								assert.Nil(t, headlessService, "Headless service should not be created when not clustering")
							}

							if assert.NotNil(t, privateService, "private service not found") {
								actual, err := roundTrip(privateService)
								if assert.NoError(t, err) {

									expected := `---
										apiVersion: v1
										kind: Service
										metadata:
											name: myrole-tor
										spec:
											ports:
											-
												name: http
												port: 80
												protocol: TCP
												targetPort: 8080
											-
												name: https
												port: 443
												protocol: TCP
												targetPort: 443
											selector:
												skiff-role-name: myrole
												skiff-role-active: "true"
									`
									testhelpers.IsYAMLEqualString(assert.New(t), expected, actual)
								}
							}

							if assert.NotNil(t, publicService, "public service not found") {
								actual, err := roundTrip(publicService)
								if assert.NoError(t, err) {
									expected := `---
										apiVersion: v1
										kind: Service
										metadata:
											name: myrole-tor-public
										spec:
											externalIPs: [ 192.0.2.42 ]
											ports:
											-
												name: https
												port: 443
												protocol: TCP
												targetPort: 443
											selector:
												skiff-role-name: myrole
												skiff-role-active: "true"
									`
									switch variant {
									case withHelmLoadBalancer:
										expected = strings.Replace(expected, "externalIPs: [ 192.0.2.42 ]", "type: LoadBalancer", 1)
									case withKube:
										expected = strings.Replace(expected, "192.0.2.42", "192.168.77.77", 1)
									}
									testhelpers.IsYAMLEqualString(assert.New(t), expected, actual)
								}
							}

						})
					}(clustering)
				}
			})
		}(variant)
	}
}
