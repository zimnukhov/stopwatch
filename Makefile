INSTALL_DIR=/usr/local/stopwatch
UI_DIR=$(INSTALL_DIR)/ui
BIN_DIR=/usr/local/bin
CONFIG_PATH=$(INSTALL_DIR)/stopwatch.conf

all:
	go build -o stopwatch

install: stopwatch
	mkdir -p $(INSTALL_DIR)
	mkdir -p $(UI_DIR)/js
	mkdir -p $(UI_DIR)/css
	cp ui/*.html $(UI_DIR)/
	cp ui/css/stopwatch.css $(UI_DIR)/css/
	cp ui/js/*.js $(UI_DIR)/js/
	mkdir -p $(BIN_DIR)
	cp stopwatch $(INSTALL_DIR)/
	ln -sf $(INSTALL_DIR)/stopwatch $(BIN_DIR)/stopwatch
ifeq ($(wildcard $(CONFIG_PATH)),)
	stopwatch -default-config >$(CONFIG_PATH)
endif

clean: 
	rm stopwatch
