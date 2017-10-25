package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pborman/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "go-acc <packages...>",
	Short: "Receive accurate code coverage reports for Golang (Go)",
	Example: `$ go-acc github.com/some/package
$ go-acc -o my-coverfile.txt github.com/some/package
$ go-acc ./...
$ go-acc $(glide novendor)`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			return
		}

		mode, err := cmd.Flags().GetString("covermode")
		if err != nil {
			fatalf("%s", err)
		}

		payload := "mode: " + mode + "\n"
		newArgs := []string{}
		for _, a := range args {
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

				newArgs = append(newArgs, add...)
			} else {
				newArgs = append(newArgs, a)
			}
		}

		files := make([]string, len(newArgs))
		for k, a := range newArgs {
			files[k] = filepath.Join(os.TempDir(), uuid.New()) + ".cc.tmp"
			c := exec.Command("go", "test", "-covermode="+mode, "-short", "-coverprofile="+files[k], "-coverpkg="+strings.Join(newArgs, ","), a)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Stdin = os.Stdin
			if err := c.Run(); err != nil {
				fatalf("%s", err)
			}
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
