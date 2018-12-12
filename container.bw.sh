aws dynamodb batch-write-item \
    --request-items file://data/container.data.json \
    --return-consumed-capacity TOTAL
