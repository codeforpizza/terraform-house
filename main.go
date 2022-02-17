package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/terraform-exec/tfexec"
)

//* handler function for each endpoint
func greet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World! %s", time.Now())
}

func createFile(w http.ResponseWriter, r *http.Request) {
	fmt.Println("triggering file creation ... ")
	pathToProcess := r.URL.Query().Get("path")
	if err := precheck(pathToProcess); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	content := r.URL.Query().Get("content")
	if err := createTerraformFile(pathToProcess, content); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("failed to run create %s", err.Error())))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("succesfully writing for path %s, content %s", pathToProcess, content)))

}

func stats(w http.ResponseWriter, r *http.Request) {
	fmt.Println("showing stats ...")
}

func main() {
	fmt.Println("starting web server")

	http.HandleFunc("/", greet)
	http.HandleFunc("/create", createFile)
	http.HandleFunc("/stats", stats)
	http.ListenAndServe(":8080", nil)
}

func precheck(path string) error {
	// * precheck case #1 sequence of number couldn't not be a path
	if _, err := strconv.Atoi(path); err == nil {
		return errors.New("could not continue precheck, case #1 detected")
	}
	return nil
}

// --------------------
type FileContent struct {
	Content  string `json:"content"`
	Filename string `json:"filename"`
}

type File struct {
	Contents []FileContent `json:"file"`
}

type Resource struct {
	LocalFile []File `json:"local_file"`
}

type TerraformFile struct {
	Resource Resource `json:"resource"`
}

func createTerraformFile(path, content string) error {

	// * create name and folder for terraform file
	tmpDir, err := ioutil.TempDir("testdir", "files")
	if err != nil {
		fmt.Println("error init folder ", err.Error())
		return err
	}

	// * create the tf.json file
	tfFile := TerraformFile{
		Resource: Resource{LocalFile: []File{
			{
				Contents: []FileContent{
					{
						Content:  content,
						Filename: path,
					},
				},
			},
		},
		},
	}

	tfJson, err := json.Marshal(tfFile)
	if err != nil {
		fmt.Println("error init file ", err.Error())
		return err
	}

	fp := filepath.Join(tmpDir, "main.tf.json")
	if err := ioutil.WriteFile(fp, tfJson, 0644); err != nil {
		fmt.Println("error write file ", err.Error())
		return err
	}

	tf, err := tfexec.NewTerraform(tmpDir, "terraform")
	if err != nil {
		fmt.Println("error init terraform folder ", err.Error())
		return err
	}

	fmt.Println("successfully init terraform file")

	if err := tf.Init(context.Background()); err != nil {
		fmt.Println("error running terraform init ", err.Error())
		return err
	}

	if err := tf.Apply(context.Background()); err != nil {
		fmt.Println("error running terraform apply ", err.Error())
		return err
	}

	// * prepare redis and save the needed data to redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB,
	})

	keyContent := fmt.Sprintf("%s:status:%s", content, "OK")
	if err := rdb.Set(context.Background(), tmpDir, keyContent, 0).Err(); err != nil {
		fmt.Println("error saving to redis ", err.Error())

	}

	return nil
}
