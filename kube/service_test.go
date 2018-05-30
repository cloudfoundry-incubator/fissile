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

func serviceTestLoadRole(assert *assert.Assertions, manifestName string) (*model.RoleManifest, *model.Role) {
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
	role := manifest.LookupRole("myrole")
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

	portDef := role.Run.ExposedPorts[0]
	if !assert.NotNil(portDef) {
		return
	}
	service, err := newService(role, newServiceTypePrivate, ExportSettings{})
	require.NoError(t, err)
	require.NotNil(t, service)

	actual, err := testhelpers.RoundtripKube(service)
	require.NoError(t, err)
	testhelpers.IsYAMLSubsetString(assert, `---
		metadata:
			name: myrole
		spec:
			ports:
			-
				name: http
				port: 80
				targetPort: http
			-
				name: https
				port: 443
				targetPort: https
			selector:
				skiff-role-name: myrole
			type: ClusterIP
	`, actual)
}

func TestServiceHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)
	service, err := newService(role, newServiceTypePrivate, ExportSettings{
		CreateHelmChart: true,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	t.Run("ClusterIP", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": nil,
		}
		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole"
			spec:
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: "http"
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: "https"
				selector:
					skiff-role-name: "myrole"
				type:	ClusterIP
		`, actual)
	})

	t.Run("LoadBalancer", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": "true",
		}

		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole"
			spec:
				ports:
				-	name: "http"
					port: 80
					protocol: "TCP"
					targetPort: "http"
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: "https"
				selector:
					skiff-role-name: "myrole"
				type:	ClusterIP
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

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)

	service, err := newService(role, newServiceTypeHeadless, ExportSettings{})
	require.NoError(t, err)
	require.NotNil(t, service)

	actual, err := testhelpers.RoundtripKube(service)
	require.NoError(t, err)
	testhelpers.IsYAMLSubsetString(assert, `---
		metadata:
			name: myrole-set
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
			type: ClusterIP
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

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)

	service, err := newService(role, newServiceTypeHeadless, ExportSettings{
		CreateHelmChart: true,
	})
	require.NoError(t, err)
	require.NotNil(t, service)

	t.Run("ClusterIP", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": nil,
		}
		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-set"
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
				type:	ClusterIP
		`, actual)
	})

	t.Run("LoadBalancer", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": "true",
		}

		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-set"
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
				type:	ClusterIP
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

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)

	service, err := newService(role, newServiceTypePublic, ExportSettings{})
	require.NoError(t, err)
	require.NotNil(t, service)

	actual, err := testhelpers.RoundtripKube(service)
	require.NoError(t, err)
	testhelpers.IsYAMLSubsetString(assert, `---
		metadata:
			name: myrole-public
		spec:
			externalIPs: [ 192.168.77.77 ]
			ports:
			-
				name: https
				port: 443
				targetPort: https
			selector:
				skiff-role-name: myrole
			type: ClusterIP
	`, actual)
}

func TestPublicServiceHelm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	manifest, role := serviceTestLoadRole(assert, "exposed-ports.yml")
	if manifest == nil || role == nil {
		return
	}

	portDef := role.Run.ExposedPorts[0]
	require.NotNil(t, portDef)

	service, err := newService(role, newServiceTypePublic, ExportSettings{
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

		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-public"
			spec:
				externalIPs: "[127.0.0.1,127.0.0.2]"
				ports:
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: "https"
				selector:
					skiff-role-name: "myrole"
				type:	ClusterIP
		`, actual)
	})

	t.Run("LoadBalanced", func(t *testing.T) {
		t.Parallel()
		config := map[string]interface{}{
			"Values.services.loadbalanced": "true",
			"Values.kube.external_ips":     "[127.0.0.1,127.0.0.2]",
		}

		actual, err := testhelpers.RoundtripNode(service, config)
		require.NoError(t, err)
		testhelpers.IsYAMLEqualString(assert, `---
			apiVersion: "v1"
			kind: "Service"
			metadata:
				name: "myrole-public"
			spec:
				ports:
				-	name: "https"
					port: 443
					protocol: "TCP"
					targetPort: "https"
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
	require.NotEmpty(t, role.Run.ExposedPorts, "Role has no exposed ports")
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
						return testhelpers.RoundtripKube(node)
					case withHelm:
						config := map[string]interface{}{
							"Values.kube.external_ips": []string{"192.0.2.42"},
						}
						return testhelpers.RoundtripNode(node, config)
					case withHelmLoadBalancer:
						config := map[string]interface{}{
							"Values.kube.external_ips":     []string{"192.0.2.42"},
							"Values.services.loadbalanced": "true",
						}
						return testhelpers.RoundtripNode(node, config)
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
								case "myrole-set":
									headlessService = service
								case "myrole":
									privateService = service
								case "myrole-public":
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
												name: myrole-set
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
												type: ClusterIP
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
											name: myrole
										spec:
											ports:
											-
												name: http
												port: 80
												protocol: TCP
												targetPort: http
											-
												name: https
												port: 443
												protocol: TCP
												targetPort: https
											selector:
												skiff-role-name: myrole
												skiff-role-active: "true"
											type: ClusterIP
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
											name: myrole-public
										spec:
											externalIPs: [ 192.0.2.42 ]
											ports:
											-
												name: https
												port: 443
												protocol: TCP
												targetPort: https
											selector:
												skiff-role-name: myrole
												skiff-role-active: "true"
											type: ClusterIP
									`
									switch variant {
									case withHelmLoadBalancer:
										expected = strings.Replace(expected, "type: ClusterIP", "type: LoadBalancer", 1)
										expected = strings.Replace(expected, "externalIPs: [ 192.0.2.42 ]", "", 1)
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
