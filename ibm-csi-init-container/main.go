/*******************************************************************************
 * IBM Confidential
 * OCO Source Materials
 * IBM Cloud Container Service, 5737-D43
 * (C) Copyright IBM Corp. 2024, All Rights Reserved.
 * The source code for this program is not  published or otherwise divested of
 * its trade secrets, irrespective of what has been deposited with
 * the U.S. Copyright Office.
 ******************************************************************************/
// Package main ...
package main

import (
	"context"
	"flag"
	"os"
	"strings"

	k8sUtils "github.com/IBM/secret-utils-lib/pkg/k8s_utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

func init() {
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis. Error cannot be usefully handled.
	logger = setUpLogger()
	defer logger.Sync() //nolint: errcheck
}

const (
	controllerName  = "ibm-vpc-block-csi-controller"
	nameSpace       = "kube-system"
	controllerLabel = "app=ibm-vpc-block-csi-driver"
)

var (
	driverVersion = flag.String("driverVersion", "", "5.1 or 5.2")
	kubeConfig    = flag.String("kubeConfig", "", "If not provide in cluster config will be considered")
	logger        *zap.Logger
)

func main() {
	flag.Parse()
	handle(logger)
	os.Exit(0)
}

func setUpLogger() *zap.Logger {
	// Prepare a new logger
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	), zap.AddCaller()).With(zap.String("name", "kube-client"))

	atom.SetLevel(zap.InfoLevel)
	return logger
}

func handle(logger *zap.Logger) {
	controllerExists := true

	logger.Info("Starting kube-client")

	// Setup Cloud Provider
	k8sClient, err := k8sUtils.Getk8sClientSet()

	if err != nil {
		logger.Fatal("Error getting k8s client", zap.Error(err))
	}

	if *driverVersion == "5.1" {
		deploymentsClient := k8sClient.Clientset.AppsV1().Deployments(nameSpace)

		if _, err := deploymentsClient.Get(context.TODO(), controllerName, metav1.GetOptions{}); err != nil {
			logger.Warn("Failed to find deployment, checking if any controller pods are running", zap.Error(err))
			controllerExists = false
		}

		if controllerExists {
			//Delete Deployment
			cleanupVPCBlockCSIControllerDeployment(deploymentsClient, logger)
		}

		// Now wait until all existing ibm-vpc-block-csi-controller pods are deleted
		checkIfControllerPodExists(k8sClient.Clientset, logger)

	} else if *driverVersion == "5.2" {
		statefulSetsClient := k8sClient.Clientset.AppsV1().StatefulSets(nameSpace)

		if _, err := statefulSetsClient.Get(context.TODO(), controllerName, metav1.GetOptions{}); err != nil {
			logger.Warn("Failed to find Statefulset, checking if any controller pods are running", zap.Error(err))
			controllerExists = false
		}

		if controllerExists {
			//Delete Statefulset
			cleanupVPCBlockCSIControllerStatefulset(statefulSetsClient, logger)
		}

		// Now wait until all existing ibm-vpc-block-csi-controller pods are deleted
		checkIfControllerPodExists(k8sClient.Clientset, logger)
	} else {
		logger.Fatal("Invalid driverVersion. Possible options 5.1 or 5.2")
	}
}

func cleanupVPCBlockCSIControllerDeployment(deploymentsClient v1.DeploymentInterface, logger *zap.Logger) {
	// Delete the Deployment
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete(context.TODO(), controllerName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		logger.Fatal("Failed to delete deployment", zap.Error(err))
	}
	logger.Info("Deployment deleted successfully")
}

func cleanupVPCBlockCSIControllerStatefulset(statefulSetsClient v1.StatefulSetInterface, logger *zap.Logger) {
	// Delete the Statefulset
	deletePolicy := metav1.DeletePropagationForeground
	if err := statefulSetsClient.Delete(context.TODO(), controllerName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		logger.Fatal("Failed to delete statefulSet", zap.Error(err))
	}
	logger.Info("StatefulSet deleted successfully")
}

func checkIfControllerPodExists(clientset kubernetes.Interface, logger *zap.Logger) {
	controllerExists := false

	for {
		pods, getPodErr := listPodsByLabel(clientset, nameSpace, "app=ibm-vpc-block-csi-driver")
		if getPodErr != nil {
			logger.Fatal("ERROR in fetching the controller pods", zap.Error(getPodErr))
		}

		for _, pod := range pods.Items {
			if strings.HasPrefix(pod.Name, controllerName) {
				controllerExists = true
				logger.Fatal("ibm-vpc-block-csi-controller controller pods still exists. Init container will continue to check for this until these are cleanedup", zap.Error(getPodErr))
			}
			if !controllerExists {
				logger.Info("All existing ibm-vpc-block-csi-controller pod deleted successfully")
				break
			}
		}
	}
}

func listPodsByLabel(k8sclient kubernetes.Interface, namespace string, label string) (*corev1.PodList, error) {
	var labelSelector metav1.ListOptions
	if label != "" {
		labelSelector = metav1.ListOptions{LabelSelector: label}
	} else {
		labelSelector = metav1.ListOptions{}
	}
	pods, err := k8sclient.CoreV1().Pods(namespace).List(context.TODO(), labelSelector)
	return pods, err
}
