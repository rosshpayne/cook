aws dynamodb batch-write-item \
    --request-items file://data/activity.data.20.7.json \
    --return-consumed-capacity TOTAL
  
aws dynamodb batch-write-item \
    --request-items file://data/activity.data.20.7a.json \
    --return-consumed-capacity TOTAL  

