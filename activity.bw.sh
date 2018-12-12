aws dynamodb batch-write-item \
    --request-items file://data/activity.data.json \
    --return-consumed-capacity TOTAL
    
aws dynamodb batch-write-item \
    --request-items file://data/activity.data.2.json \
    --return-consumed-capacity TOTAL
    
aws dynamodb batch-write-item \
    --request-items file://data/activity.data.3.json \
    --return-consumed-capacity TOTAL
