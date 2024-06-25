# Distributed Logger

Using Go, `1.26.6`, we can run a central logging server with any number of clients. Clients send logged messages thru `stdin` to the logger server. 