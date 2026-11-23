// Package version хранит единственный источник версии приложения.
//
// Значение подставляется при сборке: -ldflags "-X awg31/internal/version.Version=1.2.3".
// Без ldflags остаётся "dev", так что расхождение версии между кодом, образом и
// документацией невозможно: версия берётся из git-тега в CI.
package version

// Version — версия сборки.
var Version = "dev"
