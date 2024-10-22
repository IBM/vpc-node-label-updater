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
	"fmt"
	"os"
	"strconv"
	"strings"

	k8sUtils "github.com/IBM/secret-utils-lib/pkg/k8s_utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
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
	controllerName      = "ibm-vpc-block-csi-controller"
	nameSpace           = "kube-system"
	ssctrlPod           = "ibm-vpc-block-csi-controller-0"
	controllerLabel     = "app=ibm-vpc-block-csi-driver"
	driverContainerName = "iks-vpc-block-driver"
	vpcBlock51          = 5.1
	vpcBlock52          = 5.2
)

var (
	driverVersion = flag.String("driverVersion", "5.2", "Possible values 5.1, 5.2 or greater")
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
	), zap.AddCaller()).With(zap.String("name", "csi-init-container"))

	atom.SetLevel(zap.InfoLevel)
	return logger
}

func handle(logger *zap.Logger) {
	logger.Info("Starting csi-init-container")

	version, err := strconv.ParseFloat(*driverVersion, 64)
	if err != nil {
		logger.Warn("error in parsing driver version value", zap.Error(err), zap.Float64("version", version))
		logger.Fatal("Please check if any older version VPC Block CSI Driver version is running. Please disable and enable the VPC Block CSI Driver.If error persists open support ticket")
	}

	// Setup Cloud Provider
	k8sClient, err := k8sUtils.Getk8sClientSet()

	if err != nil {
		logger.Warn("Error getting k8s client", zap.Error(err))
		logger.Fatal("Please check if any older version VPC Block CSI Driver version is running. Please disable and enable the VPC Block CSI Driver.If error persists open support ticket")
	}

	// In case deploying version is 5.1 then we need to clean the deployment which belongs to 5.2 or later version
	if version == vpcBlock51 {
		//Delete Deployment
		cleanupVPCBlockCSIControllerDeployment(k8sClient.Clientset.AppsV1().Deployments(nameSpace), logger)
		//Delete if any leftover Deployment controller pod
		cleanupDepPod(k8sClient.Clientset, logger)

		logger.Info("csi-init-container started successfully, there exists no 5.2 VPC Block CSI Controller Deployment Pods.")

	} else if version >= vpcBlock52 { // In case deploying version is 5.2 then we need to clean the statefulset which belongs to 5.1 or earlier version
		//Delete StatefulSet
		cleanupVPCBlockCSIControllerStatefulset(k8sClient.Clientset.AppsV1().StatefulSets(nameSpace), logger)
		//Delete if any leftover StatefulSet controller pod
		cleanupCtrlPod(k8sClient.Clientset, ssctrlPod, logger)

		logger.Info("csi-init-container started successfully, there exists no 5.1 or earlier VPC Block CSI Controller Statefulset Pods.")

	} else {
		logger.Fatal("Please check if any older version VPC Block CSI Driver version is running. Please disable and enable the VPC Block CSI Driver.If error persists open support ticket")
	}
}

func cleanupVPCBlockCSIControllerDeployment(deploymentsClient v1.DeploymentInterface, logger *zap.Logger) {
	// Delete the Deployment
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete(context.TODO(), controllerName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("Deployment not found which is expected case", zap.String("Deployment", controllerName))
		} else {
			logger.Fatal("Failed to delete deployment, please cleanup the deployment manually so that VPC Block CSI Driver is up and running. Run command with admin access \"kubectl delete deployment -n kube-system ibm-vpc-block-csi-controller\"", zap.String("Deployment", controllerName), zap.Error(err))
		}
	} else {
		logger.Info("Deployment deleted successfully")
	}
}

func cleanupVPCBlockCSIControllerStatefulset(statefulSetsClient v1.StatefulSetInterface, logger *zap.Logger) {
	// Delete the Statefulset
	deletePolicy := metav1.DeletePropagationForeground
	if err := statefulSetsClient.Delete(context.TODO(), controllerName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("StatefulSet not found which is expected case", zap.String("StatefulSet", controllerName))
		} else {
			logger.Fatal("Failed to delete statefulSet, please cleanup the statefulSet manually so that VPC Block CSI Driver is up and running. Run command with admin access \"kubectl delete statefulset -n kube-system ibm-vpc-block-csi-controller\"", zap.String("StatefulSet", controllerName), zap.Error(err))
		}
	} else {
		logger.Info("StatefulSet deleted successfully")
	}
}

// Delete controller POD created by deployment or statefulset
func cleanupCtrlPod(clientset kubernetes.Interface, ctrPodName string, logger *zap.Logger) {
	if err := clientset.CoreV1().Pods(nameSpace).Delete(context.TODO(), ctrPodName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("CSI Controller pod not found which is expected case", zap.String("ctrPodName", ctrPodName))
		} else {
			errStr := fmt.Sprintf("Failed to deleted CSI Controller pod, please cleanup the pod manually so that VPC Block CSI Driver is up and running. Run command \"kubectl delete pod -n kube-system %s\"", ctrPodName)
			logger.Fatal(errStr, zap.Error(err))
		}
	} else {
		logger.Info("Pod deleted successfully", zap.String("ctrPodName", ctrPodName))
	}
}

func cleanupDepPod(clientset kubernetes.Interface, logger *zap.Logger) {
	logger.Info("Listing controller pods based on label", zap.String("controllerLabel", controllerLabel))
	pods, getPodErr := listPodsByLabels(clientset, nameSpace, controllerLabel)
	if getPodErr != nil {
		logger.Fatal("ERROR in fetching the VPC Block CSI Controller pods, Please cleanup the pods manually so that VPC Block CSI Driver is up and running. Run command \"kubectl delete pod -n kube-system ibm-vpc-block-csi-controller-xxx\"", zap.Error(getPodErr))
	}

	podUID := os.Getenv("POD_UID")
	for _, pod := range pods.Items {
		logger.Info("Pod Details", zap.String("podName", pod.Name), zap.String("podUID", string(pod.UID)))
		//Check if pod has controller prefix and the UID is not matching with upcoming Statefulset Pod
		if strings.HasPrefix(pod.Name, controllerName) && podUID != "" && podUID != string(pod.UID) {
			//Try to clean up the deployment csi controller pod
			cleanupCtrlPod(clientset, pod.Name, logger)
		}
	}
}

func listPodsByLabels(k8sclient kubernetes.Interface, namespace string, label string) (*corev1.PodList, error) {
	labelSelector := metav1.ListOptions{LabelSelector: label}
	pods, err := k8sclient.CoreV1().Pods(namespace).List(context.TODO(), labelSelector)
	return pods, err
}
