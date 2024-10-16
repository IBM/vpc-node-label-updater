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

	k8sUtils "github.com/IBM/secret-utils-lib/pkg/k8s_utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	_ = flag.Set("logtostderr", "true") // #nosec G104: Attempt to set flags for logging to stderr only on best-effort basis. Error cannot be usefully handled.
	logger = setUpLogger()
	defer logger.Sync() //nolint: errcheck
}

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

	logger.Info("Starting kube-client")
	// Setup Cloud Provider
	k8sClient, err := k8sUtils.Getk8sClientSet()

	if err != nil {
		logger.Error("Error getting k8s client", zap.Error(err))
		return
	}

	if *driverVersion == "5.1" {
		deploymentsClient := k8sClient.Clientset.AppsV1().Deployments("kube-system")
		deletePolicy := metav1.DeletePropagationForeground
		if err := deploymentsClient.Delete(context.TODO(), "demo-deployment", metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			logger.Fatal("Failed to delete deployment", zap.Error(err))
		}

		logger.Info("Deployment deleted successfully")
	} else if *driverVersion == "5.2" {
		statefulSetsClient := k8sClient.Clientset.AppsV1().StatefulSets("kube-system")
		deletePolicy := metav1.DeletePropagationForeground
		if err := statefulSetsClient.Delete(context.TODO(), "demo-deployment", metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			logger.Fatal("Failed to delete statefulSet", zap.Error(err))
		}
		logger.Info("StatefulSet deleted successfully")

	}
}

/*
func cleanupVPCBlockCSIControllerDeployment(client rest.Interface, logger *zap.Logger) {

	deploymentsClient := client.AppsV1().Deployments("kube-system")
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete(context.TODO(), "demo-deployment", metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		logger.Fatal("Failed to delete deployment", zap.Error(err))
		return err
	}

	logger.Info("Deployment deleted successfully")
}
*/
