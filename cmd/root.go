package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime/trace"
	"strings"

	"github.com/gookit/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"

	"github.com/khulnasoft/blazedock/pkg/blazedock"
)

const (
	// EnvvarWorkspaceRoot names the environment variable we check for the workspace root path
	EnvvarWorkspaceRoot = "BLAZEDOCK_WORKSPACE_ROOT"

	// EnvvarRemoteCacheBucket configures a bucket name. This enables the use of RemoteStorage
	EnvvarRemoteCacheBucket = "BLAZEDOCK_REMOTE_CACHE_BUCKET"

	// EnvvarRemoteCacheStorage configures a Remote Storage Provider. Default is GCP
	EnvvarRemoteCacheStorage = "BLAZEDOCK_REMOTE_CACHE_STORAGE"
)

const (
	bashCompletionFunc = `__blazedock_parse_get()
{
    local blazedock_output out
    if blazedock_output=$(blazedock collect 2>/dev/null); then
        out=($(echo "${blazedock_output}" | awk '{print $1}'))
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__blazedock_get_resource()
{
    __blazedock_parse_get
    if [[ $? -eq 0 ]]; then
        return 0
    fi
}

__blazedock_custom_func() {
    case ${last_command} in
        blazedock_build | blazedock_describe)
            __blazedock_get_resource
            return
            ;;
        *)
            ;;
    esac
}
`
)

var (
	workspace string
	buildArgs []string
	verbose   bool
	variant   string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "blazedock",
	Short: "A caching meta-build system",
	Long: color.Render(`<light_yellow>Blazedock is a heavily caching build system</> for Go, Yarn and Docker projects. It knows three core concepts:
  Workspace: the workspace is the root of all operations. All component names are relative to this path. No relevant
             file must be placed outside the workspace. The workspace root is marked with a WORKSPACE file.
  Component: a component is single piece of standalone software. Every folder in the workspace which contains a BUILD file
             is a component. Components are identifed by their path relative to the workspace root.
  Package:   packages are the buildable unit in blazedock. Every component can define multiple packages in its build file.
             Packages are identified by their name prefixed with the component name, e.g. some-component:pkg

<white>Configuration</>
Blazedock is configured exclusively through the WORKSPACE/BUILD files and environment variables. The following environment
variables have an effect on blazedock:
       <light_blue>BLAZEDOCK_WORKSPACE_ROOT</>  Contains the path where to look for a WORKSPACE file. Can also be set using --workspace.
 <light_blue>BLAZEDOCK_REMOTE_CACHE_STORAGE</>  Defines the remote caching storage provider. Valid values are "GCP" and "AWS". Defaults to "GCP".
  <light_blue>BLAZEDOCK_REMOTE_CACHE_BUCKET</>  Enables remote caching using GCP or S3 buckets. Required credentials depend on the storage provider:
                             - GCP: blazedock expects "gsutil" in the path configured and authenticated so that it can work with the bucket.
                             - AWS: blazedock expects that AWS credentials have been provided and with read/write access to the S3 bucket.
                               For details on configuring AWS credentials see https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html
            <light_blue>BLAZEDOCK_CACHE_DIR</>  Location of the local build cache. The directory does not have to exist yet.
            <light_blue>BLAZEDOCK_BUILD_DIR</>  Working location of blazedock (i.e. where the actual builds happen). This location will see heavy I/O
                              which makes it advisable to place this on a fast SSD or in RAM.
           <light_blue>BLAZEDOCK_YARN_MUTEX</>  Configures the mutex flag blazedock will pass to yarn. Defaults to "network".
                              See https://yarnpkg.com/lang/en/docs/cli/#toc-concurrency-and-mutex for possible values.
  <light_blue>BLAZEDOCK_DEFAULT_CACHE_LEVEL</>  Sets the default cache level for builds. Defaults to "remote".
         <light_blue>BLAZEDOCK_EXPERIMENTAL</>  Enables experimental blazedock features and commands.
`),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			log.SetLevel(log.DebugLevel)
		}
	},
	BashCompletionFunction: bashCompletionFunc,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	tp := os.Getenv("BLAZEDOCK_TRACE")
	if tp != "" {
		f, err := os.OpenFile(tp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			log.WithError(err).Fatal("cannot start trace but BLAZEDOCK_TRACE is set")
			return
		}
		defer f.Close()
		err = trace.Start(f)
		if err != nil {
			log.WithError(err).Fatal("cannot start trace but BLAZEDOCK_TRACE is set")
			return
		}
		defer trace.Stop()

		defer trace.StartRegion(context.Background(), "main").End()
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	workspaceRoot := os.Getenv(EnvvarWorkspaceRoot)
	if workspaceRoot == "" {
		var err error
		workspaceRoot, err = blazedock.DiscoverWorkspaceRoot()
		if err != nil {
			log.WithError(err).Debug("cannot determine workspace root - defaulting to .")
			workspaceRoot = "."
		} else {
			log.WithField("workspace", workspaceRoot).Debug("found workspace root")
		}
	}

	rootCmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", workspaceRoot, "Workspace root")
	rootCmd.PersistentFlags().StringArrayVarP(&buildArgs, "build-arg", "D", []string{}, "pass arguments to BUILD files")
	rootCmd.PersistentFlags().StringVar(&variant, "variant", "", "selects a package variant")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enables verbose logging")
	rootCmd.PersistentFlags().Bool("dut", false, "used for testing only - doesn't actually do anything")
}

func getWorkspace() (blazedock.Workspace, error) {
	args, err := getBuildArgs()
	if err != nil {
		return blazedock.Workspace{}, err
	}

	return blazedock.FindWorkspace(workspace, args, variant, os.Getenv("BLAZEDOCK_PROVENANCE_KEYPATH"))
}

func getBuildArgs() (blazedock.Arguments, error) {
	if len(buildArgs) == 0 {
		return nil, nil
	}

	res := make(blazedock.Arguments)
	for _, arg := range buildArgs {
		segs := strings.Split(arg, "=")
		if len(segs) < 2 {
			return nil, xerrors.Errorf("invalid build argument (format is key=value): %s", arg)
		}
		res[segs[0]] = strings.Join(segs[1:], "=")
	}
	return res, nil
}

func addExperimentalCommand(parent, child *cobra.Command) {
	if os.Getenv("BLAZEDOCK_EXPERIMENTAL") != "true" {
		return
	}

	parent.AddCommand(child)
}
