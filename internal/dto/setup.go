package dto

// SetupStatus is the progress of the installation wizard.
type SetupStatus struct {
	// Installed reports whether the application serves the requests.
	Installed bool `json:"installed"`
	// Stage is the next step of the wizard, empty once installed.
	Stage string `json:"stage,omitempty" enums:"database,admin"`
	// PasswordMinLength is the minimum length of the admin password.
	PasswordMinLength int `json:"passwordMinLength,omitempty"`
} // @name SetupStatus

// SetupServiceCheck is the connection test result of a service.
type SetupServiceCheck struct {
	OK bool `json:"ok"`
	// Version is the server version, it can be empty even if connected.
	Version string `json:"version"`
	// Message explains the failure in the language of the request, empty if OK.
	Message string `json:"message"`
} // @name SetupServiceCheck

// SetupDatabaseCheck is the connection test result of the services configured in the database step.
type SetupDatabaseCheck struct {
	Postgres SetupServiceCheck `json:"postgres"`
	Redis    SetupServiceCheck `json:"redis"`
} // @name SetupDatabaseCheck
