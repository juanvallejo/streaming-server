ENTRY_FILE=cmd/streaming.go
DEST=bin/streaming

all:
	go build -o ${DEST} ${ENTRY_FILE}

clean:
	rm -f ${DEST}
