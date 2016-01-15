package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"encoding/json"
	"io/ioutil"
	"net/http"
	"os/exec"
	"text/template"
)

type Kudd struct {
	listen      string
	kubectlPath string
}

type QuayWebhook struct {
	Repository string   `json:"repository"`
	Namespace  string   `json:"namespace"`
	Name       string   `json:"name"`
	DockerURL  string   `json:"docker_url"`
	Homepage   string   `json:"homepage"`
	Visibility string   `json:"visibility"`
	BuildId    string   `json:"build_id"`
	BuildName  string   `json:"build_name"`
	DockerTags []string `json:"docker_tags"`

	TriggerKind     string `json:"trigger_kind"`
	TriggerId       string `json:"trigger_id"`
	TriggerMetadata struct {
		Commit string `json:"commit"`
	} `json:"trigger_metadata"`
}

type TemplateData struct {
	DeploymentName string
	Image          string
	Commit         string
}

func main() {
	fs := flag.NewFlagSet("kudd", flag.ExitOnError)
	listen := fs.String("listen", ":9000", "interface and port to bind to")
	kubectl := fs.String("kubectl-path", "kubectl", "path to kubectl")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	k := &Kudd{
		listen:      *listen,
		kubectlPath: *kubectl,
	}

	http.HandleFunc("/push", k.push)
	log.Fatal(k.start())
}

func (k *Kudd) start() error {
	log.Println("serving on", k.listen)
	return http.ListenAndServe(k.listen, nil)
}

func (k *Kudd) push(w http.ResponseWriter, r *http.Request) {
	d, err := parseWebhook(r.Body)
	if err != nil {
		writeError(w, err, http.StatusBadRequest, "unable to parse webhook")
		return
	}

	resourceTemplateUrl := r.URL.Query().Get("resource_url")
	managedTag := r.URL.Query().Get("managed_tag")
	deploymentName := r.URL.Query().Get("deployment_name")
	commit := d.TriggerMetadata.Commit

	isManaged := false
	for _, tag := range d.DockerTags {
		if tag == managedTag {
			isManaged = true
			break
		}
	}
	if !isManaged {
		log.Println("This build is not managed. Not deploying")
		return
	}

	image := "quay.io/" + d.Repository + ":" + managedTag
	log.Println("image: ", image)

	resource, err := getDeploymentSpec(resourceTemplateUrl, deploymentName, image, commit)
	if err != nil {
		writeError(w, err, http.StatusBadRequest, "unable to get resource file")
		return
	}

	err = k.deployResource(image, resource)
	if err != nil {
		writeError(w, err, http.StatusNoContent, "unable to deploy resource")
		return
	}
}

func getDeploymentSpec(templateUrl, deploymentName, image, commit string) (resource string, err error) {
	res, err := http.Get(templateUrl)
	if err != nil {
		return
	}

	defer res.Body.Close()
	tmpl, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	resourceTemplate, err := template.New("resource").Parse(string(tmpl))
	if err != nil {
		return
	}

	data := TemplateData{
		DeploymentName: deploymentName,
		Image:          image,
		Commit:         commit,
	}

	var b bytes.Buffer
	err = resourceTemplate.Execute(&b, data)
	if err != nil {
		return
	}
	resource = b.String()
	return
}

func parseWebhook(r io.Reader) (v *QuayWebhook, err error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var d QuayWebhook
	err = json.Unmarshal(b, &d)
	return &d, err
}

func (k *Kudd) deployResource(image, resource string) error {
	identifier := strings.Replace(image, ":", "-", -1)
	identifier = strings.Replace(identifier, "/", "-", -1)
	log.Println("Deploying resource: ", resource)
	tempFile := fmt.Sprintf("/tmp/resource-%s.yaml", identifier)
	if err := ioutil.WriteFile(tempFile, []byte(resource), 0644); err != nil {
		log.Println("unable to write file", err)
		return err
	}

	// run kubectl create -f tempFile
	cmd := exec.Command(k.kubectlPath, "create", "-f", tempFile, "--validate=false")
	log.Println("Creating: ", cmd)
	if err := cmd.Run(); err == nil {
		log.Println("create successful")
		return nil
	} else {
		log.Println("create failed. ", err)
	}

	// run kubectl update -f tempFile if fail
	log.Println("Updating...", cmd)
	cmd = exec.Command(k.kubectlPath, "apply", "-f", tempFile, "--validate=false")
	if err := cmd.Run(); err == nil {
		log.Println("create successful")
		return nil
	} else {
		log.Println("update failed")
		return err
	}

}

func writeError(w http.ResponseWriter, err error, statusCode int, message string) {
	log.Println(message, err.Error())
	w.WriteHeader(statusCode)
	w.Write([]byte(message + ": " + err.Error()))
}
