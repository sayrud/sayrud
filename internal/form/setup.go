package form

type SetupPostgres struct {
	Host     string `json:"host" valid:"required;maxlen:253"`
	Port     int    `json:"port" valid:"min:1;max:65535"`
	Database string `json:"database" valid:"required;maxlen:63"`
	Username string `json:"username" valid:"required;maxlen:63"`
	Password string `json:"password" valid:"maxlen:256"`
	SSLMode  string `json:"sslMode" valid:"required;list:disable,require,verify-ca,verify-full" enums:"disable,require,verify-ca,verify-full"`
} // @name SetupPostgres

type SetupRedis struct {
	// Address is host:port.
	Address  string `json:"address" valid:"required;maxlen:261"`
	Username string `json:"username" valid:"maxlen:256"`
	Password string `json:"password" valid:"maxlen:512"`
} // @name SetupRedis

type SetupDatabase struct {
	Postgres SetupPostgres `json:"postgres"`
	Redis    SetupRedis    `json:"redis"`
} // @name SetupDatabase

type SetupAdmin struct {
	Email    string `json:"email" valid:"required;email;maxlen:254"`
	UserName string `json:"userName" valid:"required;maxlen:32"`
	// Password is limited to 64 characters; the setup handler also enforces bcrypt's 72-byte limit.
	Password string `json:"password" valid:"required;minlen:8;maxlen:64"`
} // @name SetupAdmin
