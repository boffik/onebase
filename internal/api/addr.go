package api

// Secure-by-default bind (план 53, этап 4; анализ §2.7): раньше сервер слушал
// :port на всех интерфейсах, а при отсутствии пользователей auth выключен
// целиком — проброс порта наружу открывал базу и консоль кода без пароля.
// Теперь по умолчанию 127.0.0.1; наружу — только явным --host.

import (
	"net"
	"strconv"
	"strings"
)

// listenAddr строит адрес прослушивания; пустой host = loopback.
func listenAddr(host string, port int) string {
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// IsLoopbackHost сообщает, является ли host локальным (loopback) адресом.
// Используется CLI для предупреждения при старте наружу без пользователей.
func IsLoopbackHost(host string) bool {
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
