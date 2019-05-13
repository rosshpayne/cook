aws dynamodb batch-write-item \
    --request-items file://data/activity.data.21.8.json \
    --return-consumed-capacity TOTAL
    