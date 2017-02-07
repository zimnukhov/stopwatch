INSTALL_DIR=/usr/local/stopwatch
BIN_DIR=/usr/local/bin
CONFIG_PATH=$(INSTALL_DIR)/stopwatch.conf

all:
	go build -o stopwatch

install: stopwatch
	mkdir -p $(INSTALL_DIR)
	mkdir -p $(BIN_DIR)
	cp stopwatch $(INSTALL_DIR)/
	ln -sf $(INSTALL_DIR)/stopwatch $(BIN_DIR)/stopwatch
ifeq ($(wildcard $(CONFIG_PATH)),)
	stopwatch -default-config >$(CONFIG_PATH)
endif

clean: 
	rm stopwatch
