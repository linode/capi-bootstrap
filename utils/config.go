package utils

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/clientcmd/api"
	apiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

// MergeAPIConfigIntoV1Config merges a clientcmd api.Config created via clientcmd.Load into a clientcmd v1 Config.
// Set updateContext if the incoming api.Config should be set as the CurrentContext in the v1 Config that this function returns.
func MergeAPIConfigIntoV1Config(apiConfig *api.Config, v1Config *apiv1.Config, updateContext bool) (*apiv1.Config, error) {
	for k, v := range apiConfig.AuthInfos {
		apiAuthInfo := apiv1.AuthInfo{}
		rawV1AuthInfo, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(rawV1AuthInfo, &apiAuthInfo)
		if err != nil {
			return nil, err
		}

		newAuthInfo := apiv1.NamedAuthInfo{
			Name:     k,
			AuthInfo: apiAuthInfo,
		}
		for _, authInfo := range v1Config.AuthInfos {
			if !reflect.DeepEqual(authInfo, newAuthInfo) {
				v1Config.AuthInfos = append(v1Config.AuthInfos, newAuthInfo)
				continue
			}
		}
	}

	for k, v := range apiConfig.Clusters {
		apiCluster := apiv1.Cluster{}
		rawV1Cluster, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(rawV1Cluster, &apiCluster)
		if err != nil {
			return nil, err
		}

		newCluster := apiv1.NamedCluster{
			Name:    k,
			Cluster: apiCluster,
		}
		for _, cluster := range v1Config.Clusters {
			if !reflect.DeepEqual(cluster, newCluster) {
				v1Config.Clusters = append(v1Config.Clusters, newCluster)
				continue
			}
		}
	}

	for k, v := range apiConfig.Contexts {
		apiContext := apiv1.Context{}
		rawV1Context, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(rawV1Context, &apiContext)
		if err != nil {
			return nil, err
		}

		newContext := apiv1.NamedContext{
			Name:    k,
			Context: apiContext,
		}
		for _, v1Context := range v1Config.Contexts {
			if !reflect.DeepEqual(v1Context, newContext) {
				v1Config.Contexts = append(v1Config.Contexts, newContext)
				continue
			}
		}
	}

	for k, v := range apiConfig.Extensions {
		ext := runtime.RawExtension{}
		rawExt, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(rawExt, &ext)
		if err != nil {
			return nil, err
		}
		newExt := apiv1.NamedExtension{
			Name:      k,
			Extension: ext,
		}
		for _, extension := range v1Config.Extensions {
			if !reflect.DeepEqual(extension, newExt) {
				v1Config.Extensions = append(v1Config.Extensions, newExt)
				continue
			}
		}
	}

	if updateContext {
		v1Config.CurrentContext = apiConfig.CurrentContext
	}

	v1Config.APIVersion = apiv1.SchemeGroupVersion.Version
	v1Config.Kind = "Config"
	return v1Config, nil
}
