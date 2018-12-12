aws dynamodb create-table \
    --table-name Sessions  \
    --attribute-definitions \
        AttributeName=Sid,AttributeType=S \
    --key-schema \
        AttributeName=Sid,KeyType=HASH \
    --provisioned-throughput \
        ReadCapacityUnits=1,WriteCapacityUnits=1