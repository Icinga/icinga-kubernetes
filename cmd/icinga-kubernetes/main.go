/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/icinga/icinga-kubernetes/pkg/controller"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/jmoiron/sqlx"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func createClientSet() (*kubernetes.Clientset, error) {
	var kubeconfig string
	var master string

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error getting user home dir: %v\n", err)
		os.Exit(1)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.StringVar(&kubeconfig, "kubeconfig", kubeConfigPath, "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	return clientset, nil
}

func main() {
	var dbConfig string
	flag.StringVar(&dbConfig, "dbConfig", "./config.yml", "path to database config file")
	flag.Parse()

	clientset, _ := createClientSet()

	// TODO: Create database from a YAML configuration file.*/*/
	d, err := database.FromYAMLFile(dbConfig)
	if err != nil {
		log.Fatal(err)
	}

	dsn := mysql.Config{
		User:                 d.User,
		Passwd:               d.Password,
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(d.Host, fmt.Sprint(3306)),
		DBName:               d.Database,
		AllowNativePasswords: true,
		Params:               map[string]string{"sql_mode": "ANSI_QUOTES"},
	}

	db, err := sqlx.Open("mysql", dsn.FormatDSN())
	if err != nil {
		klog.Fatal(err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)

	stop := make(chan struct{})
	defer close(stop)

	podInformer := factory.Core().V1().Pods().Informer()
	podSync := controller.NewPodSync(clientset, db)
	podSync.WarmUp(podInformer.GetIndexer())
	{
		c := controller.NewController(podInformer, podSync.Sync)
		go c.Run(1, stop)
	}

	nodeInformer := factory.Core().V1().Nodes().Informer()
	nodeSync := controller.NewNodeSync(db)
	nodeSync.WarmUp(nodeInformer.GetIndexer())
	{
		c := controller.NewController(nodeInformer, nodeSync.Sync)
		go c.Run(1, stop)
	}

	deploymentInformer := factory.Apps().V1().Deployments().Informer()
	deploymentSync := controller.NewDeploymentSync(db)
	deploymentSync.WarmUp(deploymentInformer.GetIndexer())
	{
		c := controller.NewController(deploymentInformer, deploymentSync.Sync)
		go c.Run(1, stop)
	}

	// Wait forever
	select {}
}
