curl -k -X POST -H 'Content-Type: application/x-www-form-urlencoded' -d 'grant_type=client_credentials&client_id=amzn1.application-oa2-client.dd124c1834a2412f9a697c2059835e87&client_secret=1ff6ed545f7e4c8d267c2ebe4bbf2f22ee291a888514740db39dcde9e9d40f72&scope=alexa:skill_messaging' https://api.amazon.com/auth/O2/token
# response
# {
#     "access_token":"Atc|MQEBIPAA8EvA3LqOPmFQmOF_OaXhF7FOPGyQqBlpNbvtsy0orU7CWQtIUcUxn2_o_5ghWqAqsIvAAWoRln0gN6nMVe3eCvOje2Z9Tm6FcMjK_kuYdKe58sAHMzhHoPkvwhGEMZB5kBzRaje6oLJGH7bWzij3QBuf-CJh3c7_AF6nkpg3IxQo3X-jun_ICM7eqdAkBOI4T5ajQ85OrcBRRWJkZVDZ56hBSkB1emtCPjkaQnwZYBAoWAyI7fq8EkUeBr0dsnoQmMaSzwM6mOCC8RBB6bhwZ_yBbgbhQvpsDK539x--gw"
#     ,"scope":"alexa:skill_messaging"
#     ,"token_type":"bearer"
#     ,"expires_in":3600
# }