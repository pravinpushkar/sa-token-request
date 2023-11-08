package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	str2duration "github.com/xhit/go-str2duration/v2"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
)

var (
	k8sSecretName      string        = "gateway-sa-secret"
	serviceAccountName string        = "higress-gateway"
	namespace          string        = "higress-system"
	tokenAudience      string        = "istio-ca"
	tokenExpiryTime    time.Duration = time.Second * 31536000 // 365 days
)

func main() {
	readEnv()

	klog.InitFlags(nil)
	flag.Parse()

	var config *rest.Config
	var err error

	if config, err = rest.InClusterConfig(); err != nil {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig := filepath.Join(home, ".kube", "config")
			config, _ = clientcmd.BuildConfigFromFlags("", kubeconfig)
		} else {
			panic(fmt.Errorf("unable to load kubeconfig: %w", err))
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(fmt.Errorf("unable to create clientset: %w", err))
	}

	secret, err := createSecret(clientset)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		fmt.Println("Secret already exists, getting the current secret")
		secret, err = getSecret(clientset)
		if err != nil {
			panic(fmt.Errorf("unable to get Secret: %w", err))
		}
	} else if err != nil {
		panic(fmt.Errorf("unable to create Secret: %w", err))
	}

	expirationSeconds := int64(tokenExpiryTime.Seconds())
	treq := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         []string{tokenAudience},
			ExpirationSeconds: &expirationSeconds,
			BoundObjectRef: &authenticationv1.BoundObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Name:       k8sSecretName,
			},
		},
	}

	tokenReq, err := clientset.CoreV1().ServiceAccounts(namespace).CreateToken(context.Background(), serviceAccountName, treq, metav1.CreateOptions{})
	if err != nil {
		panic(fmt.Errorf("unable to create token: %w", err))
	}
	fmt.Println("Token created")
	token := strings.TrimSpace(tokenReq.Status.Token)

	secret.Data = map[string][]byte{
		"token": []byte(token),
	}

	_, err = updateSecret(clientset, secret)

	if err != nil && strings.Contains(err.Error(), "the object has been modified") {
		fmt.Println("Secret has been modified, getting the current secret")
		latestSecret, err := getSecret(clientset)
		if err != nil {
			panic(fmt.Errorf("unable to get Secret: %w", err))
		}
		secret.ResourceVersion = latestSecret.ResourceVersion
		fmt.Println("Retying updating secret")
		_, err = updateSecret(clientset, secret)
		if err != nil {
			panic(fmt.Errorf("unable to update Secret: %w", err))
		}
	} else if err != nil {
		panic(fmt.Errorf("unable to update Secret: %w", err))
	}
	fmt.Println("Secret updated")
}

func readEnv() {
	if val, ok := os.LookupEnv("SECRET_NAME_FOR_GW_TOKEN"); !ok {
		fmt.Println("SECRET_NAME_FOR_GW_TOKEN env variable not set, using default value: ", k8sSecretName)
	} else {
		k8sSecretName = val
		fmt.Println("SECRET_NAME_FOR_GW_TOKEN: ", k8sSecretName)
	}

	if val, ok := os.LookupEnv("SERVICE_ACCOUNT_NAME"); !ok {
		fmt.Println("SERVICE_ACCOUNT_NAME env variable not set, using default value: ", serviceAccountName)
	} else {
		serviceAccountName = val
		fmt.Println("SERVICE_ACCOUNT_NAME: ", serviceAccountName)
	}

	if val, ok := os.LookupEnv("NAMESPACE"); !ok {
		fmt.Println("NAMESPACE env variable not set, using default value: ", namespace)
	} else {
		namespace = val
		fmt.Println("NAMESPACE: ", namespace)
	}

	if val, ok := os.LookupEnv("TOKEN_AUDIENCE"); !ok {
		fmt.Println("TOKEN_AUDIENCE env variable not set, using default value: ", tokenAudience)
	} else {
		tokenAudience = val
		fmt.Println("TOKEN_AUDIENCE: ", tokenAudience)
	}

	if val, ok := os.LookupEnv("TOKEN_EXPIRATION_SECONDS"); !ok {
		fmt.Println("TOKEN_EXPIRATION_SECONDS env variable not set, using default value: ", tokenExpiryTime)
	} else {
		tokenExpiration, err := str2duration.ParseDuration(val)
		if err != nil {
			fmt.Println("TOKEN_EXPIRATION_SECONDS parse error, using default value: ", tokenExpiryTime)
		} else {
			tokenExpiryTime = tokenExpiration
		}
		fmt.Println("TOKEN_EXPIRATION: ", tokenExpiryTime)
	}
}

func createSecret(clientset *kubernetes.Clientset) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sSecretName,
			Namespace: namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": serviceAccountName,
			},
		},
		Data: map[string][]byte{
			"token": []byte(""),
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	secret, err := clientset.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create Secret: %w", err)
	}
	return secret, nil
}

func getSecret(clientset *kubernetes.Clientset) (*corev1.Secret, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), k8sSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get Secret: %w", err)
	}
	return secret, nil
}

func updateSecret(clientset *kubernetes.Clientset, secret *corev1.Secret) (*corev1.Secret, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to update Secret: %w", err)
	}
	return secret, nil
}
