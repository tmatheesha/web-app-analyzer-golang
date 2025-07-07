package env

import "github.com/spf13/viper"

type Config struct {
	LogLevel                string `mapstructure:"LOG_LEVEL"`
	Port                    string `mapstructure:"PORT"`
	NumOfWorkers            int    `mapstructure:"NUM_OF_WORKERS"`
	ContextTimeoutInSeconds int    `mapstructure:"CONTEXT_TIMEOUT_SECONDS"`
	WebAppTitle             string `mapstructure:"WEB_APP_TITLE"`
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigType("env")
	viper.SetConfigName("app")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
