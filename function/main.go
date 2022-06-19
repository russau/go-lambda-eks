package main

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"log"

	runtime "github.com/aws/aws-lambda-go/lambda"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	clusterIDHeader = "x-k8s-aws-id"
	v1Prefix        = "k8s-aws-v1."
)

type FunctionEvent struct {
	ClusterName string
}

var clusterCache map[string]clientcmdapi.Cluster

func getClusterInfo(clusterName string) (clientcmdapi.Cluster, error) {
	////////////////// EKS stuff
	cfg, _ := awsconfig.LoadDefaultConfig(context.TODO())
	eksClient := eks.NewFromConfig(cfg)
	clusterInfo, err := eksClient.DescribeCluster(context.TODO(), &eks.DescribeClusterInput{
		Name: &clusterName,
	})
	if err != nil {
		return clientcmdapi.Cluster{}, err
	}
	cert, _ := b64.RawStdEncoding.DecodeString(*clusterInfo.Cluster.CertificateAuthority.Data)
	return clientcmdapi.Cluster{
		Server:                   *clusterInfo.Cluster.Endpoint,
		CertificateAuthorityData: cert,
	}, nil
}

func getAuthToken(clusterName string) string {
	///////////////// STS STUFF
	cfg, _ := awsconfig.LoadDefaultConfig(context.TODO())
	stsClient := sts.NewFromConfig(cfg)
	stsSigner := sts.NewPresignClient(stsClient)
	presignedURLRequest, _ := stsSigner.PresignGetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{}, func(presignOptions *sts.PresignOptions) {
		presignOptions.ClientOptions = append(presignOptions.ClientOptions, func(stsOptions *sts.Options) {
			// Add clusterId Header
			stsOptions.APIOptions = append(stsOptions.APIOptions, smithyhttp.SetHeaderValue(clusterIDHeader, clusterName))
			// Add X-Amz-Expires query param
			stsOptions.APIOptions = append(stsOptions.APIOptions, smithyhttp.SetHeaderValue("X-Amz-Expires", "60"))
		})
	})
	return v1Prefix + b64.RawURLEncoding.EncodeToString([]byte(presignedURLRequest.URL))
}

func handleRequest(ctx context.Context, event FunctionEvent) (string, error) {
	log.Println("Getting cluster")
	clusterName := event.ClusterName
	cluster, ok := clusterCache[clusterName]
	if !ok {
		log.Println("Cache miss - calling EKS")
		var err error
		cluster, err = getClusterInfo(clusterName)
		if err != nil {
			return "An error ocurred", err
		}
		clusterCache[clusterName] = cluster
	}

	/// configure kubernetes client
	config := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster1": &cluster,
		},
		Contexts: map[string]*clientcmdapi.Context{
			"context1": {
				Cluster:  "cluster1",
				AuthInfo: "context1",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"context1": {},
		},
		CurrentContext: "context1",
	}
	log.Println("Getting token")
	config.AuthInfos[config.CurrentContext].Token = getAuthToken(clusterName)

	// create the clientset
	rawConfig, _ := clientcmd.NewDefaultClientConfig(config, &clientcmd.ConfigOverrides{}).ClientConfig()
	clientset, _ := kubernetes.NewForConfig(rawConfig)

	// get the list of pods
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "An error ocurred", err
	}
	return fmt.Sprintf("There are %d pods in the cluster", len(pods.Items)), nil
}

func main() {
	clusterCache = make(map[string]clientcmdapi.Cluster)
	runtime.Start(handleRequest)
}
