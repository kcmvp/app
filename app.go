package app

import (
	_ "embed"
	"fmt"
	"github.com/kcmvp/app/util"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	cfgOpt  mo.Option[*viper.Viper]
	injOpt  mo.Option[do.Injector]
	rootDir string
)

const (
	defaultCfgName = "application"
)

func init() {
	dir, _ := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").CombinedOutput()
	rootDir = util.CleanStr(string(dir))
	if len(rootDir) == 0 {
		rootDir = mo.TupleToResult(os.Executable()).MustGet()
	}
	// init project config
	cfg := viper.New()
	cfg.SetConfigName(defaultCfgName) // name of cfg file (without extension)
	cfg.SetConfigType("yaml")         // REQUIRED if the cfg file does not have the extension in the name
	cfg.AddConfigPath(rootDir)        // optionally look for cfg in the working directory
	cfgOpt = mo.Some(cfg)
	if err := cfg.ReadInConfig(); err != nil { // Find and read the cfg file
		log.Println("Warning: no configuration file found")
		cfgOpt = mo.None[*viper.Viper]()
	}
	injOpt = mo.None[do.Injector]()
	if cfgOpt.IsPresent() && util.ActiveProfile().Test() {
		tCfg := viper.New()
		tCfg.SetConfigName(fmt.Sprintf("%s_test.yaml", defaultCfgName)) // name of cfg file (without extension)
		tCfg.SetConfigType("yaml")                                      // REQUIRED if the cfg file does not have the extension in the name
		tCfg.AddConfigPath(rootDir)                                     // optionally look for cfg in the working directory
		if err := tCfg.ReadInConfig(); err != nil {
			panic(fmt.Errorf("failed to merge test configuration file: %w", err))
		}
		rootKeys := lo.Uniq(lo.Map(tCfg.AllKeys(), func(key string, index int) string {
			return strings.Split(key, ".")[0]
		}))
		patch := map[string]any{}
		lo.ForEach(cfg.AllKeys(), func(key string, _ int) {
			if lo.NoneBy(rootKeys, func(root string) bool {
				return strings.HasPrefix(key, root)
			}) {
				patch[key] = cfg.Get(key)
			}
		})
		if err := tCfg.MergeConfigMap(patch); err != nil {
			panic(fmt.Errorf("failed to merge test configuration file: %w", err))
		}
		cfgOpt = mo.Some(tCfg)
	}
	if cfgOpt.IsPresent() {
		injOpt = mo.Some[do.Injector](do.NewWithOpts(&do.InjectorOpts{
			HookAfterRegistration: []func(scope *do.Scope, serviceName string){
				func(scope *do.Scope, serviceName string) {
					fmt.Printf("scope is %s, name is %s \n", scope.Name(), serviceName)
				},
			},
			Logf: func(format string, args ...any) {
				log.Printf(format, args...)
			},
		}))
	}
}

func Cfg() mo.Option[*viper.Viper] {
	return cfgOpt
}

func RootDir() string {
	return rootDir
}

func Container() mo.Option[do.Injector] {
	return injOpt
}

type ContextAware func(*viper.Viper) func(do.Injector)

//func Context(service ContextAware) func(do.Injector) {
//	return service(App().cfg)
//}
//
//func Register(servers ...func(do.Injector)) {
//	do.Package(servers...)(App())
//}
