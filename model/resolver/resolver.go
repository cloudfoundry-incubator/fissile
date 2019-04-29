package resolver

import (
	"fmt"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/util"
	"code.cloudfoundry.org/fissile/validation"
	yaml "gopkg.in/yaml.v2"
)

type internalVariable struct {
	CVOptions model.CVOptions `yaml:"options"`
}

type internalVariableDefinitions struct {
	Variables []internalVariable `yaml:"variables"`
}

// Resolver prepares, calculates and resolves the manifest
type Resolver struct {
	roleManifest    *model.RoleManifest
	releaseResolver model.ReleaseResolver
	options         model.LoadRoleManifestOptions
}

// NewResolver returns a new resolver
func NewResolver(
	m *model.RoleManifest,
	releaseResolver model.ReleaseResolver,
	options model.LoadRoleManifestOptions,
) *Resolver {
	return &Resolver{
		roleManifest:    m,
		releaseResolver: releaseResolver,
		options:         options,
	}
}

// Resolve pre-processes the manifest calls ResolveRoleManifest() as well as
// ResolveLinks()
func (r *Resolver) Resolve() (*model.RoleManifest, error) {
	var err error
	m := r.roleManifest
	// Releases
	m.LoadedReleases, err = r.releaseResolver.Load(
		r.options.ReleaseOptions,
		m.Releases,
	)
	if err != nil {
		return nil, err
	}

	// Configuration Templates
	if m.Configuration == nil {
		m.Configuration = &model.Configuration{}
	}

	if m.Configuration.RawTemplates == nil {
		m.Configuration.RawTemplates = yaml.MapSlice{}
	}

	// Parse CVOptions
	var definitions internalVariableDefinitions
	err = yaml.Unmarshal(m.ManifestContent, &definitions)
	if err != nil {
		return nil, err
	}

	for i, v := range definitions.Variables {
		m.Variables[i].CVOptions = v.CVOptions
	}

	// Resolve manifest
	err = r.ResolveRoleManifest()
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ResolveRoleManifest takes a role manifest and validates
// it to ensure it has no errors, and that the various ancillary structures are
// correctly populated.
// This method was made public so tests can have their own package and we avoid import cycles.
func (r *Resolver) ResolveRoleManifest() error {
	m := r.roleManifest
	grapher := r.options.Grapher
	allErrs := validation.ErrorList{}

	// If template keys are not strings, we need to stop early to avoid panics
	allErrs = append(allErrs, validateTemplateKeysAndValues(m)...)
	if len(allErrs) != 0 {
		return allErrs
	}

	err := r.releaseResolver.MapReleases(m.LoadedReleases)
	if err != nil {
		return err
	}

	if grapher != nil {
		for _, release := range m.LoadedReleases {
			grapher.GraphNode("release/"+release.Name, map[string]string{"label": "release/" + release.Name})
		}
	}

	if m.Configuration.Authorization.Accounts == nil {
		m.Configuration.Authorization.Accounts = make(map[string]model.AuthAccount)
	}

	if m.Configuration.Authorization.RoleUsedBy == nil {
		m.Configuration.Authorization.RoleUsedBy = make(map[string]map[string]struct{})
	}

	if m.Configuration.Authorization.ClusterRoleUsedBy == nil {
		m.Configuration.Authorization.ClusterRoleUsedBy = make(map[string]map[string]struct{})
	}

	for _, instanceGroup := range m.InstanceGroups {
		// Don't allow any instance groups that are not of the "bosh" or "bosh-task" type
		// Default type is considered to be "bosh".
		// The kept instance groups are validated.
		switch instanceGroup.Type {
		case "":
			instanceGroup.Type = model.RoleTypeBosh
		case model.RoleTypeBosh, model.RoleTypeBoshTask, model.RoleTypeColocatedContainer:
			// Nothing to do.
		default:
			allErrs = append(allErrs, validation.Invalid(
				fmt.Sprintf("instance_groups[%s].type", instanceGroup.Name),
				instanceGroup.Type, "Expected one of bosh, bosh-task, or colocated-container"))
		}

		// default_feature, if_feature, and unless_feature all all mutually exclusive
		if (instanceGroup.DefaultFeature != "" && (instanceGroup.IfFeature != "" || instanceGroup.UnlessFeature != "")) ||
			(instanceGroup.IfFeature != "" && instanceGroup.UnlessFeature != "") {

			allErrs = append(allErrs, validation.Forbidden(
				fmt.Sprintf("instance_groups[%s]", instanceGroup.Name),
				fmt.Sprintf("default_feature[%s], if_feature[%s], and unless_feature[%s] are all mutually exclusive",
					instanceGroup.DefaultFeature, instanceGroup.IfFeature, instanceGroup.UnlessFeature)))
		}

		m.AddFeature(instanceGroup.DefaultFeature, true)
		m.AddFeature(instanceGroup.IfFeature, false)
		m.AddFeature(instanceGroup.UnlessFeature, false)

		allErrs = append(allErrs, instanceGroup.CalculateRoleRun()...)
		allErrs = append(allErrs, validateRoleTags(instanceGroup)...)
		allErrs = append(allErrs, validateRoleRun(instanceGroup, m)...)
		allErrs = append(allErrs, validateJobReferences(instanceGroup)...)

		// Count how many instance groups use a particular
		// service account. And its roles.

		accountName := "default"
		if instanceGroup.Run != nil {
			accountName = instanceGroup.Run.ServiceAccount
		}
		account := m.Configuration.Authorization.Accounts[accountName]

		for _, roleName := range account.Roles {
			if m.Configuration.Authorization.RoleUsedBy[roleName] == nil {
				m.Configuration.Authorization.RoleUsedBy[roleName] = make(map[string]struct{})
			}
			m.Configuration.Authorization.RoleUsedBy[roleName][accountName] = struct{}{}
		}
		for _, clusterRoleName := range account.ClusterRoles {
			if m.Configuration.Authorization.ClusterRoleUsedBy[clusterRoleName] == nil {
				m.Configuration.Authorization.ClusterRoleUsedBy[clusterRoleName] = make(map[string]struct{})
			}
			m.Configuration.Authorization.ClusterRoleUsedBy[clusterRoleName][accountName] = struct{}{}
		}
	}

	if len(allErrs) != 0 {
		return allErrs
	}

	for _, instanceGroup := range m.InstanceGroups {
		instanceGroup.SetRoleManifest(m)
		errorList := validateInstanceGroup(m, instanceGroup, r.releaseResolver)
		if len(errorList) != 0 {
			allErrs = append(allErrs, errorList...)
		}

		if grapher != nil {
			for _, jobReference := range instanceGroup.JobReferences {
				if jobReference.Job != nil {
					grapher.GraphNode(jobReference.Job.Fingerprint, map[string]string{"label": "job/" + jobReference.Job.Name})
				}
			}
		}
	}

	// Skip further validation if we fail to resolve any jobs
	// This lets us assume valid jobs in the validation routines
	if len(allErrs) == 0 {
		r.calculateConfigurationTemplates(m)

		if !r.releaseResolver.CanValidate() {
			allErrs = append(allErrs, r.ResolveLinks()...)
		}
		allErrs = append(allErrs, validateVariableType(m.Variables)...)
		allErrs = append(allErrs, validateVariableSorting(m.Variables)...)
		allErrs = append(allErrs, validateVariablePreviousNames(m.Variables)...)
		allErrs = append(allErrs, validateServiceAccounts(m)...)
		allErrs = append(allErrs, validateUnusedColocatedContainerRoles(m)...)
		allErrs = append(allErrs, validateColocatedContainerPortCollisions(m)...)
		allErrs = append(allErrs, validateColocatedContainerVolumeShares(m)...)
		allErrs = append(allErrs, validateVariableDescriptions(m)...)
		if !r.releaseResolver.CanValidate() {
			allErrs = append(allErrs, validateScripts(m, r.options.ValidationOptions)...)
		}
		allErrs = append(allErrs, resolvePodSecurityPolicies(m)...)
	}

	if len(allErrs) != 0 {
		return allErrs
	}

	return nil
}

// ResolveLinks examines the BOSH links specified in the job specs and maps
// them to the correct role / job that can be looked up at runtime.
// This method was made public so tests can have their own package and we avoid import cycles.
func (r *Resolver) ResolveLinks() validation.ErrorList {
	m := r.roleManifest
	errors := make(validation.ErrorList, 0)

	// Build mappings of providers by name, and by type.  Note that the names
	// involved here are the aliases, where appropriate.
	providersByName := make(map[string]model.JobProvidesInfo)
	providersByType := make(map[string][]model.JobProvidesInfo)
	for _, instanceGroup := range m.InstanceGroups {
		for _, jobReference := range instanceGroup.JobReferences {
			var availableProviders []string
			serviceName := jobReference.ContainerProperties.BoshContainerization.ServiceName
			if serviceName == "" {
				serviceName = fmt.Sprintf("%s-%s", util.ConvertNameToKey(instanceGroup.Name), util.ConvertNameToKey(jobReference.Name))
			}
			for availableName, availableProvider := range jobReference.Job.AvailableProviders {
				availableProviders = append(availableProviders, availableName)
				if availableProvider.Type != "" {
					providersByType[availableProvider.Type] = append(providersByType[availableProvider.Type], model.JobProvidesInfo{
						JobLinkInfo: model.JobLinkInfo{
							Name:        availableProvider.Name,
							Type:        availableProvider.Type,
							RoleName:    instanceGroup.Name,
							JobName:     jobReference.Name,
							ServiceName: serviceName,
						},
						Properties: availableProvider.Properties,
					})
				}
			}
			for name, provider := range jobReference.ExportedProvides {
				info, ok := jobReference.Job.AvailableProviders[name]
				if !ok {
					errors = append(errors, validation.NotFound(
						fmt.Sprintf("instance_groups[%s].jobs[%s].provides[%s]", instanceGroup.Name, jobReference.Name, name),
						fmt.Sprintf("Provider not found; available providers: %v", availableProviders)))
					continue
				}
				if provider.Alias != "" {
					name = provider.Alias
				}
				providersByName[name] = model.JobProvidesInfo{
					JobLinkInfo: model.JobLinkInfo{
						Name:        info.Name,
						Type:        info.Type,
						RoleName:    instanceGroup.Name,
						JobName:     jobReference.Name,
						ServiceName: serviceName,
					},
					Properties: info.Properties,
				}
			}
		}
	}

	// Resolve the consumers
	for _, instanceGroup := range m.InstanceGroups {
		for _, jobReference := range instanceGroup.JobReferences {
			expectedConsumers := make([]model.JobConsumesInfo, len(jobReference.Job.DesiredConsumers))
			copy(expectedConsumers, jobReference.Job.DesiredConsumers)
			// Deal with any explicitly marked consumers in the role manifest
			for consumerName, consumerInfo := range jobReference.ResolvedConsumes {
				consumerAlias := consumerName
				if consumerInfo.Alias != "" {
					consumerAlias = consumerInfo.Alias
				}
				if consumerAlias == "" {
					// There was a consumer with an explicitly empty name
					errors = append(errors, validation.Invalid(
						fmt.Sprintf(`instance_group[%s].job[%s]`, instanceGroup.Name, jobReference.Name),
						"name",
						fmt.Sprintf("consumer has no name")))
					continue
				}
				provider, ok := providersByName[consumerAlias]
				if !ok {
					errors = append(errors, validation.NotFound(
						fmt.Sprintf(`instance_group[%s].job[%s].consumes[%s]`, instanceGroup.Name, jobReference.Name, consumerName),
						fmt.Sprintf(`consumer %s not found`, consumerAlias)))
					continue
				}
				jobReference.ResolvedConsumes[consumerName] = model.JobConsumesInfo{
					JobLinkInfo: provider.JobLinkInfo,
				}

				for i := range expectedConsumers {
					if expectedConsumers[i].Name == consumerName {
						expectedConsumers = append(expectedConsumers[:i], expectedConsumers[i+1:]...)
						break
					}
				}
			}
			// Handle any consumers not overridden in the role manifest
			for _, consumerInfo := range expectedConsumers {
				// Consumers don't _have_ to be listed; they can be automatically
				// matched to a published name, or to the only provider of the
				// same type in the whole deployment
				var provider model.JobProvidesInfo
				var ok bool
				if consumerInfo.Name != "" {
					provider, ok = providersByName[consumerInfo.Name]
				}
				if !ok && len(providersByType[consumerInfo.Type]) == 1 {
					provider = providersByType[consumerInfo.Type][0]
					ok = true
				}
				if ok {
					name := consumerInfo.Name
					if name == "" {
						name = provider.Name
					}
					info := jobReference.ResolvedConsumes[name]
					info.Name = provider.Name
					info.Type = provider.Type
					info.RoleName = provider.RoleName
					info.JobName = provider.JobName
					info.ServiceName = provider.ServiceName
					jobReference.ResolvedConsumes[name] = info
				} else if !consumerInfo.Optional {
					errors = append(errors, validation.Required(
						fmt.Sprintf(`instance_group[%s].job[%s].consumes[%s]`, instanceGroup.Name, jobReference.Name, consumerInfo.Name),
						fmt.Sprintf(`failed to resolve provider %s (type %s)`, consumerInfo.Name, consumerInfo.Type)))
				}
			}
		}
	}

	errors = append(errors, r.recordJobConsumers(m)...)

	return errors
}

// recordJobConsumers examines a role manifest and records in each job what
// roles consume it.
func (r *Resolver) recordJobConsumers(m *model.RoleManifest) validation.ErrorList {
	errors := make(validation.ErrorList, 0)

	for _, consumerInstanceGroup := range m.InstanceGroups {
		for _, consumerJob := range consumerInstanceGroup.JobReferences {
			for linkName, consumer := range consumerJob.ResolvedConsumes {
				providerInstanceGroup := m.LookupInstanceGroup(consumer.RoleName)
				if providerInstanceGroup == nil {
					// This should not happen: we resolved a link, but can no
					// longer find the instance group that provides it.
					field := fmt.Sprintf("instance_group[%s].job[%s].consumes[%s]", consumerInstanceGroup.Name, consumerJob.Name, linkName)
					message := fmt.Errorf("Could not find resolved instance group %s", consumer.RoleName)
					errors = append(errors, validation.InternalError(field, message))
					continue
				}
				providerJob := providerInstanceGroup.LookupJob(consumer.JobName)
				if providerJob == nil {
					// This should not happen: we resolved a link, but can no
					// longer find the job that provides it.
					field := fmt.Sprintf("instance_group[%s].job[%s].consumes[%s]", consumerInstanceGroup.Name, consumerJob.Name, linkName)
					message := fmt.Errorf("Could not find resolved job %s in instance group %s", consumer.JobName, consumer.RoleName)
					errors = append(errors, validation.InternalError(field, message))
					continue
				}
				linkInfo := model.JobLinkInfo{
					Name:        consumer.Name,
					Type:        consumer.Type,
					RoleName:    consumerInstanceGroup.Name,
					JobName:     consumerJob.Name,
					ServiceName: consumer.ServiceName, // unused
				}
				providerJob.ResolvedConsumedBy[linkName] = append(providerJob.ResolvedConsumedBy[linkName], linkInfo)
			}
		}
	}

	return errors
}

// resolvePodSecurityPolicies ensures that the pod security policies (PSPs) are
// in a sane state:
// - RoleManifest.Configuration.Authorization.Accounts can reference multiple cluster-roles
// - The cluster roles are specified in
//   RoleManifest.Configuration.Authorization.ClusterRoles
// - The cluster role may be able to use PSPs
// - If no PSPs are specified for an account, then the instance group may specify if it needs to be privileged via
//   RoleManifest.InstanceGroup.JobReferences[].ContainerProperties.BoshContainerization.PodSecurityPolicy
// - If using the backwards compatibility key, then if a privileged PSP is requested but the service account is not
//   normally bound to a privileged PSP, a duplicate of the service account is created with a binding to the PSP (where
//   the name of the new service account is the old names with a "-privileged" suffix).
func resolvePodSecurityPolicies(m *model.RoleManifest) validation.ErrorList {
	errors := make(validation.ErrorList, 0)

	// Calculate what PSPs are attached to each cluster role
	// Calculate whether each service account has a privileged PSP attached
	// Find instance groups with insufficient privileges (and their service accounts)
	// If no instance groups require more privileges, stop.
	// Find a PSP to use as the privileged PSP
	// Create a cluster role to use that PSP (if necessary)
	// For each inadequate account, create a clone
	// Fix up instance groups to use the cloned accounts

	// Calculate what PSPs are attached to each cluster role
	clusterRolePSPNames := make(map[string][]string)
	for clusterRoleName, clusterRole := range m.Configuration.Authorization.ClusterRoles {
		for _, rule := range clusterRole {
			if !rule.IsPodSecurityPolicyRule() {
				continue
			}
			for _, resourceName := range rule.ResourceNames {
				if _, ok := m.Configuration.Authorization.PodSecurityPolicies[resourceName]; ok {
					clusterRolePSPNames[clusterRoleName] = append(clusterRolePSPNames[clusterRoleName], resourceName)
				} else {
					errors = append(errors, validation.NotFound(
						fmt.Sprintf(`configuration.auth.cluster-roles[%s]`, clusterRoleName),
						fmt.Sprintf(`pod security policy %s`, resourceName)))
				}
			}
		}
	}

	// Calculate whether each service account has a privileged PSP attached
	serviceAccountHasPrivilegedPSP := make(map[string]bool)
	for accountName, account := range m.Configuration.Authorization.Accounts {
		serviceAccountHasPrivilegedPSP[accountName] = false
		for clusterRoleIndex, clusterRoleName := range account.ClusterRoles {
			_, ok := m.Configuration.Authorization.ClusterRoles[clusterRoleName]
			if !ok {
				// cluster role is missing
				errors = append(errors, validation.NotFound(
					fmt.Sprintf(`configuration.auth.accounts[%s].cluster-roles[%d]`, accountName, clusterRoleIndex),
					fmt.Sprintf(`cluster role name %s`, clusterRoleName)))
				continue
			}
			for _, clusterRolePSPName := range clusterRolePSPNames[clusterRoleName] {
				psp := m.Configuration.Authorization.PodSecurityPolicies[clusterRolePSPName]
				if psp.PrivilegeEscalationAllowed() {
					serviceAccountHasPrivilegedPSP[accountName] = true
				}
				break
			}
		}
	}

	// Find instance groups with insufficient privileges (and their service accounts)
	accountsNeedingEscalation := make(map[string][]int)
	instanceGroupsNeedingEscalation := make(map[int]struct{})
	for instanceGroupIndex, instanceGroup := range m.InstanceGroups {
		serviceAccountName := instanceGroup.Run.ServiceAccount
		serviceAccount := m.Configuration.Authorization.Accounts[serviceAccountName]

		if serviceAccount.UsedBy == nil {
			serviceAccount.UsedBy = make(map[string]struct{})
		}
		serviceAccount.UsedBy[instanceGroup.Name] = struct{}{}
		m.Configuration.Authorization.Accounts[serviceAccountName] = serviceAccount

		if e := instanceGroup.EnsureValidPodSecurityPolicyPrivilege(); e != nil {
			// Instance group has invalid values
			details := e.(*model.InvalidPodSecurityPolicyPrivilegeError)
			errors = append(errors, validation.Invalid(
				fmt.Sprintf(`instance_groups[%s].jobs[%s].properties.bosh_containerization.pod-security-policy`,
					instanceGroup.Name,
					details.JobName),
				details.Value,
				fmt.Sprintf("Expected one of: %s, %s",
					model.PodSecurityPolicyNonPrivileged,
					model.PodSecurityPolicyPrivileged)))
			continue
		}

		if !instanceGroup.IsPrivilegedPodSecurityPolicy() {
			// Ignore any instance groups that don't request extra privileges
			continue
		}
		privilegedAccount, ok := serviceAccountHasPrivilegedPSP[serviceAccountName]
		if !ok {
			if serviceAccountName == "default" {
				// Create a dummy one instead
				privilegedAccount = false
			} else {
				errors = append(errors, validation.NotFound(
					fmt.Sprintf(`instance_groups[%s].run.service-account`, instanceGroup.Name),
					fmt.Sprintf(`service account %s`, serviceAccountName)))
				continue
			}
		}

		if privilegedAccount {
			// service account is already privileged, no need to escalate
			continue
		}
		accountsNeedingEscalation[serviceAccountName] = append(accountsNeedingEscalation[serviceAccountName], instanceGroupIndex)
		instanceGroupsNeedingEscalation[instanceGroupIndex] = struct{}{}
	}

	// If no instance groups require more privileges, stop.
	if len(instanceGroupsNeedingEscalation) < 1 {
		return errors
	}

	// Find a PSP to use as the privileged PSP
	defaultPrivilegedPSPName := "privileged"
	for pspName, psp := range m.Configuration.Authorization.PodSecurityPolicies {
		if util.StringInSlice(pspName, []string{"privileged", "default"}) {
			if psp.PrivilegeEscalationAllowed() {
				defaultPrivilegedPSPName = pspName
				break
			}
		}
	}

	// Create a cluster role to use that PSP (if necessary)
	var defaultPrivilegedClusterRoleName string
	if m.Configuration.Authorization.ClusterRoles == nil {
		m.Configuration.Authorization.ClusterRoles = make(map[string]model.AuthRole)
	}
clusterRoleLoop:
	for clusterRoleName, clusterRole := range m.Configuration.Authorization.ClusterRoles {
		for _, rule := range clusterRole {
			if !rule.IsPodSecurityPolicyRule() {
				continue
			}
			if util.StringInSlice(defaultPrivilegedPSPName, rule.ResourceNames) {
				defaultPrivilegedClusterRoleName = clusterRoleName
				break clusterRoleLoop
			}
		}
	}
	if defaultPrivilegedClusterRoleName == "" {
		newClusterRole := model.AuthRole{model.AuthRule{
			APIGroups:     []string{"extensions"},
			Verbs:         []string{"use"},
			Resources:     []string{"podsecuritypolicies"},
			ResourceNames: []string{defaultPrivilegedPSPName},
		}}
		defaultPrivilegedClusterRoleName = "default-privileged"
		m.Configuration.Authorization.ClusterRoles[defaultPrivilegedClusterRoleName] = newClusterRole
	}

	// For each inadequate account, create a clone (with corresponding cluster role)
	for oldAccountName, affectedInstanceGroups := range accountsNeedingEscalation {
		newAccountName := fmt.Sprintf("%s-privileged", oldAccountName)
		oldAccount := m.Configuration.Authorization.Accounts[oldAccountName]
		newAccount, ok := m.Configuration.Authorization.Accounts[newAccountName]
		if !ok {
			// The account didn't previously exist; create it by copying the old one
			newAccount = model.AuthAccount{
				Roles:        append(oldAccount.Roles),
				ClusterRoles: append(oldAccount.ClusterRoles, defaultPrivilegedClusterRoleName),
				UsedBy:       make(map[string]struct{}),
			}
			// Mark the roles as being used by the new account
			for _, roleName := range newAccount.Roles {
				m.Configuration.Authorization.RoleUsedBy[roleName][newAccountName] = struct{}{}
			}
			for _, clusterRoleName := range newAccount.ClusterRoles {
				if m.Configuration.Authorization.ClusterRoleUsedBy[clusterRoleName] == nil {
					// If we just created the default privileged cluster role, it has no counter yet
					m.Configuration.Authorization.ClusterRoleUsedBy[clusterRoleName] = make(map[string]struct{})
				}
				m.Configuration.Authorization.ClusterRoleUsedBy[clusterRoleName][newAccountName] = struct{}{}
			}
		}
		// Transfer the usage from the old account to the new account
		for _, instanceGroupIndex := range affectedInstanceGroups {
			instanceGroupName := m.InstanceGroups[instanceGroupIndex].Name
			delete(oldAccount.UsedBy, instanceGroupName)
			newAccount.UsedBy[instanceGroupName] = struct{}{}
		}
		m.Configuration.Authorization.Accounts[oldAccountName] = oldAccount
		m.Configuration.Authorization.Accounts[newAccountName] = newAccount
	}

	// Fix up instance groups to use the cloned accounts
	for instanceGroupIndex := range instanceGroupsNeedingEscalation {
		instanceGroup := m.InstanceGroups[instanceGroupIndex]
		oldAccountName := instanceGroup.Run.ServiceAccount
		newAccountName := fmt.Sprintf("%s-privileged", oldAccountName)
		instanceGroup.Run.ServiceAccount = newAccountName
	}
	return errors
}

// calculateConfigurationTemplates caculates the global configuration templates
// (only used for validation purposes) based on the configuration templates from
// the individual instance groups. The resulting set is the union of globally-
// declared templates and instance-group-specific ones.
func (r *Resolver) calculateConfigurationTemplates(m *model.RoleManifest) {

	m.Configuration.Templates = make(map[string]model.ConfigurationTemplate)

	for _, g := range m.InstanceGroups {
		for templateName, templateDef := range g.Configuration.Templates {
			_, ok := m.Configuration.Templates[templateName]
			if !ok || templateDef.IsGlobal {
				m.Configuration.Templates[templateName] = templateDef
			}
		}
	}
}
