package configs

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}
