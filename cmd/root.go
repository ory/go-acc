package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ory/x/flagx"
	"github.com/pborman/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "go-acc <flags> <packages...>",
	Short: "Receive accurate code coverage reports for Golang (Go)",
	Example: `$ go-acc github.com/some/package
$ go-acc -o my-coverfile.txt github.com/some/package
$ go-acc ./...
$ go-acc $(glide novendor)

You can pass all flags defined by "go test" after "--":
$ go-acc . -- -short -v -failfast
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			return
		}

		mode := flagx.MustGetString(cmd, "covermode")
		if flagx.MustGetBool(cmd, "verbose") {
			fmt.Println("Flag -v has been deprecated, use `go acc -- -v` instead!")
		}

		mode, err := cmd.Flags().GetString("covermode")
		if err != nil {
			fatalf("%s", err)
		}

		payload := "mode: " + mode + "\n"
		var packages []string
		var passthrough []string
		for _, a := range args {
			if len(a) > 1 && a[0] == '-' && a != "--" {
				passthrough = append(passthrough, a)
			} else {
				if len(a) > 4 && a[len(a)-4:] == "/..." {
					var buf bytes.Buffer
					c := exec.Command("go", "list", a)
					c.Stdout = &buf
					c.Stderr = &buf
					if err := c.Run(); err != nil {
						fatalf("%s", err)
					}

					add := []string{}
					for _, s := range strings.Split(buf.String(), "\n") {
						if len(s) > 0 {
							add = append(add, s)
						}
					}

					packages = append(packages, add...)
				} else {
					packages = append(packages, a)
				}
			}
		}

		files := make([]string, len(packages))
		for k, pkg := range packages {
			files[k] = filepath.Join(os.TempDir(), uuid.New()) + ".cc.tmp"
			var c *exec.Cmd
			ca := append(append(
				[]string{
					"test",
					"-covermode=" + mode,
					"-coverprofile=" + files[k],
					"-coverpkg=" + strings.Join(packages, ","),
				},
				passthrough...),
				pkg)
			c = exec.Command("go", ca...)
			//var buf bytes.Buffer
			//c.Stdout = &buf
			//c.Stderr = &buf
			//c.Stdin = os.Stdin
			//
			stderr, err := c.StderrPipe()
			if err != nil {
				fatalf("%s", err)
			}
			stdout, err := c.StdoutPipe()
			if err != nil {
				fatalf("%s", err)
			}

			if err := c.Start(); err != nil {
				fatalf("%s", err)
			}

			var wg sync.WaitGroup
			wg.Add(2)
			go scan(&wg, stderr)
			go scan(&wg, stdout)

			if err := c.Wait(); err != nil {
				fatalf("%s", err)
			}

			wg.Wait()
		}

		for _, file := range files {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				continue
			}

			p, err := ioutil.ReadFile(file)
			if err != nil {
				fatalf("%s", err)
			}

			ps := strings.Split(string(p), "\n")
			payload += strings.Join(ps[1:], "\n")
		}

		output, err := cmd.Flags().GetString("output")
		if err != nil {
			fatalf("%s", err)
		}

		if err := ioutil.WriteFile(output, []byte(payload), 0644); err != nil {
			fatalf("%s", err)
		}
	},
}

func scan(wg *sync.WaitGroup, r io.ReadCloser) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "warning: no packages being tested depend on matches for pattern") {
			continue
		}
		fmt.Println(strings.Split(line, "% of statements in")[0])
	}
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags, which, if defined here,
	// will be global for your application.

	//RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.accurate-code-coverage.yaml)")
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	RootCmd.Flags().BoolP("verbose", "v", false, "Does nothing, there for compatibility")
	RootCmd.Flags().StringP("output", "o", "coverage.txt", "Location for the output file")
	RootCmd.Flags().String("covermode", "atomic", "Which code coverage mode to use")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(".accurate-code-coverage") // name of config file (without extension)
	viper.AddConfigPath("$HOME")                   // adding home directory as first search path
	viper.AutomaticEnv()                           // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func fatalf(msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
	fmt.Println("")
	os.Exit(1)
}

type filter struct {
	dtl io.Writer
}

func (f *filter) Write(p []byte) (n int, err error) {
	for _, ppp := range strings.Split(string(p), "\n") {
		if strings.Contains(ppp, "warning: no packages being tested depend on matches for pattern") {
			continue
		} else {
			nn, err := f.dtl.Write(p)
			n = n + nn
			if err != nil {
				return n, err
			}
		}
	}
	return len(p), nil
}
