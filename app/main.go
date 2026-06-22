package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
)

func getIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "IP não encontrado"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok &&
			!ipnet.IP.IsLoopback() &&
			ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}

	return "IP não encontrado"
}

func handler(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	fmt.Fprintf(w, `
<html>
<head>
	<title>Go Demo v2</title>
</head>
<body>
	<h1>Olá Kubernetes!</h1>
	<p>Bem-vindo à aplicação Go.</p>
	<p><strong>Hostname:</strong> %s</p>
	<p><strong>IP:</strong> %s</p>
</body>
</html>
`, hostname, getIP())
}

func main() {
	http.HandleFunc("/", handler)

	fmt.Println("Servidor iniciado na porta 8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
