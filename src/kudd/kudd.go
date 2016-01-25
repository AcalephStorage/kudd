package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"

	"io/ioutil"
	"net/http"
	"os/exec"
	"text/template"
)

type Kudd struct {
	listen      string
	kubectlPath string
}

type TemplateData struct {
	Name   string
	Branch string
	Commit string
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

	defer r.Body.Close()

	query := r.URL.Query()
	name := query.Get("name")
	branch := query.Get("branch")
	commit := query.Get("commit")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeError(w, err, http.StatusNotFound, "unable to read kudd file")
		return
	}
	kuddTemplate, err := template.New("kudd").Parse(string(data))
	if err != nil {
		writeError(w, err, http.StatusBadRequest, "unable to parse kudd file")
		return
	}

	kuddData := TemplateData{
		Name:   name,
		Branch: branch,
		Commit: commit,
	}

	var b bytes.Buffer
	err = kuddTemplate.Execute(&b, kuddData)
	if err != nil {
		writeError(w, err, http.StatusNoContent, "unable to execute kudd file")
		return
	}
	kuddDeploy := b.String()

	err = k.deployResource(kuddDeploy, kuddData)
	if err != nil {
		writeError(w, err, http.StatusNoContent, "unable to deploy resource")
		return
	}
}

func (k *Kudd) deployResource(kuddSpec string, kuddMeta TemplateData) error {
	identifier := fmt.Sprintf("%s-%s-%s", kuddMeta.Name, kuddMeta.Branch, kuddMeta.Commit)
	log.Println("Deploying spec: ", identifier)

	tempFile := fmt.Sprintf("/tmp/kudd-%s.yaml", identifier)
	if err := ioutil.WriteFile(tempFile, []byte(kuddSpec), 0644); err != nil {
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
		log.Println("Update Successful")
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
