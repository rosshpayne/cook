aws dynamodb batch-write-item \
    --request-items file://data/ingredient.data.1.json \
    --return-consumed-capacity TOTAL
    
    aws dynamodb batch-write-item \
    --request-items file://data/ingredient.data.2.json \
    --return-consumed-capacity TOTAL
    
