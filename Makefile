DEPS=\
	 github.com/aws/aws-sdk-go \
	 github.com/gin-gonic/gin \
	 github.com/gin-contrib/sessions \
	 github.com/satori/go.uuid

all:
	@echo "Must specify an exact target."

deps:
	@for tool in  $(DEPS) ; do \
		echo "Installing/Updating $$tool" ; \
		go get -u $$tool; \
	done
