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
	"sync"
)

var (
	once    sync.Once
	cfg     *viper.Viper
	inj     do.Injector
	rootDir string
)

const (
	defaultCfgName = "application"
)

type Resource interface {
	Close() error
}

type Register struct {
	Name        string
	Constructor do.Provider[Resource]
}

type Provider func() []Register

func Start(providers ...Provider) {
	once.Do(func() {
		dir, _ := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").CombinedOutput()
		rootDir = util.CleanStr(string(dir))
		if len(rootDir) == 0 {
			rootDir = mo.TupleToResult(os.Executable()).MustGet()
		}
		cfg = viper.New()
		cfg.SetConfigName(defaultCfgName)
		cfg.SetConfigType("yaml")
		cfg.AddConfigPath(rootDir)
		if err := cfg.ReadInConfig(); err != nil {
			log.Println("Warning: no configuration file found")
		} else if util.ActiveProfile().Test() {
			tCfg := viper.New()
			tCfg.SetConfigName(fmt.Sprintf("%s_test.yaml", defaultCfgName))
			tCfg.SetConfigType("yaml")
			tCfg.AddConfigPath(rootDir)
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
			cfg = tCfg
		}
		inj = do.NewWithOpts(&do.InjectorOpts{
			HookAfterRegistration: []func(scope *do.Scope, serviceName string){
				func(scope *do.Scope, serviceName string) {
					fmt.Printf("scope is %s, name is %s \n", scope.Name(), serviceName)
				},
			},
			Logf: func(format string, args ...any) {
				log.Printf(format, args...)
			},
		})
		lo.ForEach(providers, func(provider Provider, _ int) {
			lo.ForEach(provider(), func(register Register, _ int) {
				do.ProvideNamed(inj, register.Name, register.Constructor)
			})
		})
	})
}

func RootDir() string {
	return rootDir
}

func Container() do.Injector {
	return inj
}

func CfgMap(name string) map[string]string {
	return cfg.GetStringMapString(name)
}
