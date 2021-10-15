FROM scratch
COPY brev-cli /
ENTRYPOINT ["/brev-cli"]
