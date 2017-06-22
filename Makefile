DEPS=\
	 github.com/aws/aws-sdk-go \
	 github.com/gin-gonic/gin \
	 gopkg.in/gin-gonic/gin.v1 \
	 github.com/gin-contrib/sessions \
	 github.com/satori/go.uuid

all:
	@echo "Must specify an exact command."

deps:
	@for tool in  $(DEPS) ; do \
		echo "Installing/Updating $$tool" ; \
		go get -u $$tool; \
	done
