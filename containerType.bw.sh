aws dynamodb batch-write-item \
    --request-items file://data/containerType.data.json \
    --return-consumed-capacity TOTAL
