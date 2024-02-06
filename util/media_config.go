package util

import (
	"github.com/spf13/viper"
)

// Config stores all configuration of the application.
// The value are read by viper from a config file or environment variables.
type MediaConfig struct {
	UseHandle string `mapstructure:"USE_HANDLE"`

	FSUpLoadDir string `mapstructure:"FS_UPLOAD_DIR"`

	S3AccessKeyId     string `mapstructure:"S3_ACCESS_KEY_ID"`
	S3SecretAccessKey string `mapstructure:"S3_SECRET_ACCESS_KEY"`
	S3Region          string `mapstructure:"S3_REGION"`
	S3ButketName      string `mapstructure:"S3_BUCKET_NAME"`
	S3DisableSSL      bool   `mapstructure:"S3_DISABLE_SSL"`
	S3ForcePathStyle  bool   `mapstructure:"S3_FORCE_PATH_STYLE"`
	S3EndPoint        string `mapstructure:"S3_END_POINT"`
	S3CorsOrigins     string `mapstructure:"S3_CORS_ORIGINS"`
}

// LoadConfig reads configuration from file or environment variable.
func LoadMediaConfig(path string) (config MediaConfig, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app-media")
	viper.SetConfigType("env")

	viper.AutomaticEnv()
	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	err = viper.Unmarshal(&config)
	return
}
