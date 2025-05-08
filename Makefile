ENTRY_FILE=cmd/streaming.go
DEST=bin/streaming

.PHONY: all

all: tools
	go build -o ${DEST} ${ENTRY_FILE}

client:
	$(MAKE) -C pkg/webclient all

tools:
	go build -o bin/analytics tools/analytics/main.go

clean:
	rm -f ${DEST}
