const Alexa = require('ask-sdk-core');
var AWS = require('aws-sdk');


//var   recipe = { Response :[] };
var   invokeParams = {
        FunctionName: 'apigw-lambda-stack-3-TestFunction-1W27R33Q8ONM2',
        Qualifier: 'dev'
};

var lambda = new AWS.Lambda();
AWS.config.region = 'us-east-1';
const querystring = require('querystring');

const messages = {
      WELCOME: 'Welcome to the Sample Device Address API Skill!  You can ask for the device address by saying what is my address.  What do you want to ask?',
      WHAT_DO_YOU_WANT: 'What do you want to ask?',
      NOTIFY_MISSING_PERMISSIONS: 'Please enable Location permissions in the Amazon Alexa app.',
      NO_ADDRESS: 'It looks like you don\'t have an address set. You can set your address from the companion app.',
      ADDRESS_AVAILABLE: 'Here is your full address: ',
      ERROR: 'Uh Oh. Looks like something went wrong.',
      LOCATION_FAILURE: 'There was an error with the Device Address API. Please try again.',
      GOODBYE: 'Bye! Thanks for using the Sample Device Address API Skill!',
      UNHANDLED: 'This skill doesn\'t support that. Please ask something else.',
      HELP: 'You can use this skill by asking something like: whats my address?',
      STOP: 'Bye! Thanks for using the Sample Device Address API Skill!',
    };

const LaunchRequestHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'LaunchRequest';
  },
  
  handle(handlerInput) {
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    const accessToken = handlerInput.requestEnvelope.context.System.apiAccessToken;
    const accessEndpoint = handlerInput.requestEnvelope.context.System.apiEndpoint;
      
    invokeParams.Payload = '{ "Path" : "start" ,"Param" : "'+uid+reqId+'" }';
    
    var promise= new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        }); 
        
    return promise.then((resp) => {
        resp = JSON.parse(resp);
        console.log("inner resp:", resp);
        if (resp.Type === 'email') {
              //await getEmail(accessToken,accessEndpoint);
              const https = require('https');
    
              const options = {
                hostname: accessEndpoint.substring(8,accessEndpoint.length),
                path: "/v2/accounts/~current/settings/Profile.email",
                //method: "GET",
                headers: {
                      "content-type": "application/json",
                      "accept": "application/json",
                      "authorization": "Bearer" + " " + accessToken
                }
              };
              var prom = new Promise((resolve, reject) => {
                    var req = https.get(options, function(res) {
                        // reject on bad status
                        if (res.statusCode < 200 || res.statusCode >= 300) {
                            return reject(new Error('statusCode=' + res.statusCode));
                        }
                        // cumulate data
                        var body = [];
                        res.on('data', function(chunk) {
                            body.push(chunk);
                        });
                        // resolve on end
                        res.on('end', function() {
                            try {
                                body=String(body);      
                            } catch(e) {
                                reject(e);
                            }
                            resolve(body);
                        });
                    });
                    // reject on request error
                    req.on('error', function(err) {
                        // This is not a "Second reject", just a different sort of failure
                        reject(err);
                    });
                    req.on('data', data=> {
                      resolve();
                    });
                    req.end();
                  });
              
              return prom.then( (data) => {
                      let email=data;
                      console.log("Email is: ",email);
                      invokeParams.Payload = '{ "Path" : "startWithEmail" ,"Param" : "'+uid+'&email=' + email.substring(1,email.length-1) + '" }';
                      
                      var promise2 = new Promise((resolve, reject) => {
                          lambda.invoke(invokeParams, function(err, data) {
                          if (err) {
                            reject(err);
                          } else {
                            resolve(data.Payload);  }
                          });
                        });
                        
                      return promise2.then((body) => {
                              var  resp = JSON.parse(body);
                              console.log("promise 2 resp: ",resp);
                              return handleResponse(handlerInput, resp);
                              
                              }).catch(function(err) { // catch invoke start with Email
                                     console.log("promise 2 error catch: ", err);
                              });
                      
              }).catch(function(err) { // catch resp from http.get email
                 //console.log("prom error catch: ");
                 const PERMISSIONS = ['alexa::profile:email:read'];
                 return handlerInput.responseBuilder
                                  .speak("please give permission to email")
                                  .withAskForPermissionsConsentCard(PERMISSIONS)
                                  .getResponse();
                });
        } else { // resp from invoke start
              console.log("about to handle response....");
              return handleResponse(handlerInput, resp);
        }
    }).catch(function(err) { // catch invoke start..
      console.log("invoke start .. error catch: ", err);
                      return handlerInput.responseBuilder
                                   .speak("Error has occured")
                                   .getResponse();
    });
            
  },
};


const TestIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'TestIntent';
  },
  handle(handlerInput) {
    //const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    // const accessToken = handlerInput.requestEnvelope.context.System.apiAccessToken;
    // const accessEndpoint = handlerInput.requestEnvelope.context.System.apiEndpoint;
    // //function setTimer(expireSec, msg, accessToken, accessEndpoint, handlerInput){
    //console.log("In TimerInternHandler accessEndpoint ",accessEndpoint);
    //setTimer(300,"Test timer today", accessToken,accessEndpoint,handlerInput);
              const tripple = require('APL/TestSpeak.js'); 
          const speakcmd = require('APL/testspeakcmd.js');
          const Text = "Separate 6 eggs.  Whites into a {addtoc.0}, and yolks, into a";
           const Verbal = "<speak>Sepperate 6 eggs.  Whites into a large bowl, and yolks, into a small bowl</speak>";
           
          return  handlerInput.responseBuilder
                            .reprompt(Verbal)
                            .addDirective(tripple("header","subhrd", Verbal,Text))
                            .addDirective(speakcmd())
                            .getResponse(); 
  },
};



function setTimer(expireSec, msg, accessToken, accessEndpoint, handlerInput){
  const https = require('https');
  const event = handlerInput.requestEnvelope;
              const request = {
                                requestTime : new Date(),
                                trigger: {
                                      type : "SCHEDULED_RELATIVE",
                                      offsetInSeconds : expireSec
                                },
                                alertInfo : {
                                      spokenInfo : {
                                          content : [{
                                              locale : event.request.locale,
                                              text : msg
                                          }]
                                      }
                                  },
                                  pushNotification : {                            
                                      status : "ENABLED"         
                                  }
                              };
              const data = JSON.stringify(request);
              
              console.log("Reminder data: ",data);

              const options = {
                hostname: accessEndpoint.substring(8,accessEndpoint.length),
                path: "/v1/alerts/reminders",
                method: "Post",
                headers: {
                      "content-type" : "application/json",
                      "authorization" : "Bearer" + " " + accessToken,
                      "content-length" : Buffer.byteLength(data)
                }
              };
              var prom= new Promise((resolve, reject) => {
                    var req = https.request(options, function(res) {
                        // reject on bad status
                        if (res.statusCode < 200 || res.statusCode >= 300) {
                            return reject(new Error('statusCode=' + res.statusCode));
                        }
                        res.setEncoding('utf8');
                        // cumulate data
                        var body = [];
                        res.on('data', function(chunk) {
                            body.push(chunk);
                        });
                        // resolve on end
                        res.on('end', function() {
                            try {
                                body=String(body);      
                            } catch(e) {
                               console.log("on res-on-end Error deteceted...");
                                reject(e);
                            }
                            resolve(body);
                        });
                    });
                    // reject on request error
                    req.on('error', function(err) {
                        // This is not a "Second reject", just a different sort of failure
                        console.log("Here in req-on-error");
                        reject(err);
                    });
                    req.on('data', data=> {
                      resolve();
                    });
                    req.write(data);
                    req.end();
                  });
              
              return prom.then( (data) => {
                  // timer set
                  console.log("response: ", JSON.stringify(data) );
                      
              }).catch(function(err) { // catch resp from http.get email
                console.log("prom error catch: ",err);
                const PERMISSIONS = ['alexa::alerts:reminders:skill:readwrite'];
                return handlerInput.responseBuilder
                                  .speak("please give permission to set a timer for you")
                                  .withAskForPermissionsConsentCard(PERMISSIONS)
                                  .getResponse();
                });
}

// async function getEmail(handlerInput) {
//         const PERMISSIONS = ['alexa::profile:email:read'];
//         const { responseBuilder, serviceClientFactory } = handlerInput;
//         try {
//                 const upsServiceClient = serviceClientFactory.getUpsServiceClient();  
//                 const email = await upsServiceClient.getProfileEmail(); 
                
//                 return email;
                
//         } catch (error) {
//                 if (error.name == 'ServiceError') {
//                   console.log('ERROR StatusCode:' + error.statusCode + ' ' + error.message);
//                 }
//                 return responseBuilder
//                   .speak(messages.NOTIFY_MISSING_EMAIL_PERMISSIONS)
//                   .withAskForPermissionsConsentCard(PERMISSIONS)
//                   .getResponse();
//         }
// }


const EventHandler = {
  canHandle: handlerInput =>
    handlerInput.requestEnvelope.request.type === 'Alexa.Presentation.APL.UserEvent',
  
  handle: handlerInput => {
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId ;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    console.log(handlerInput.requestEnvelope.request);
    console.log("reqId " , reqId);
    const args = handlerInput.requestEnvelope.request.arguments;
    const event = args[0];
    //const ordinal = args[1];
    //const data = args[2];
    // value passed from APL when user presses selectable container
    const selid='&sId='+args[1];

    switch (event) {
    case 'select':
      invokeParams.Payload = '{ "Path" : "select" ,"Param" : "'+uid+reqId+selid+'" }';

      var promise= new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });
    

    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
        
    case 'backButton':
       invokeParams.Payload = '{ "Path" : "back" ,"Param" : "'+uid+reqId+'" }';
        
       promise= new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    } 
  },
};


const GetAddressIntentHandler = {
  canHandle(handlerInput) {
    const { request } = handlerInput.requestEnvelope;

    return request.type === 'IntentRequest' && request.intent.name === 'GetAddressIntent';
  },
  async handle(handlerInput) {

    console.log('=> Get Address -> handle -> handlerInput', JSON.stringify(handlerInput));
    const PERMISSIONS = ['read::alexa:device:all:address'];
    const { requestEnvelope, serviceClientFactory, responseBuilder } = handlerInput;

    const consentToken = requestEnvelope.context.System.user.permissions
      && requestEnvelope.context.System.user.permissions.consentToken;
    if (!consentToken) {
      return responseBuilder
        .speak(messages.NOTIFY_MISSING_PERMISSIONS)
        .withAskForPermissionsConsentCard(PERMISSIONS)
        .getResponse();
    }
    try {
      const { deviceId } = requestEnvelope.context.System.device;
      const deviceAddressServiceClient = serviceClientFactory.getDeviceAddressServiceClient();
      const address = await deviceAddressServiceClient.getFullAddress(deviceId);

      console.log('Address successfully retrieved, now responding to user.');

      let response;
      if (address.addressLine1 === null && address.stateOrRegion === null) {
        response = responseBuilder.speak(messages.NO_ADDRESS).getResponse();
      } else {
        const ADDRESS_MESSAGE = `${messages.ADDRESS_AVAILABLE + address.addressLine1}, ${address.stateOrRegion}, ${address.postalCode}`;
        response = responseBuilder.speak(ADDRESS_MESSAGE).getResponse();
      }
      return response;
    } catch (error) {
      if (error.name !== 'ServiceError') {
        const response = responseBuilder.speak(messages.ERROR).getResponse();
        return response;
      }
      throw error;
    }
  },
};

const GetEmailIntentHandler = {
  canHandle(handlerInput) {
    const { request } = handlerInput.requestEnvelope;

    return request.type === 'IntentRequest' && request.intent.name === 'GetEmailIntent';
  },
  handle(handlerInput) {
    const https = require('https');
    const messages = {
      WELCOME: 'Welcome to the Sample Device Address API Skill!  You can ask for the device address by saying what is my address.  What do you want to ask?',
      WHAT_DO_YOU_WANT: 'What do you want to ask?',
      NOTIFY_MISSING_PERMISSIONS: 'Please enable Location permissions in the Amazon Alexa app.',
      NO_ADDRESS: 'It looks like you don\'t have an address set. You can set your address from the companion app.',
      ADDRESS_AVAILABLE: 'Here is your full address: ',
      ERROR: 'Uh Oh. Looks like something went wrong.',
      LOCATION_FAILURE: 'There was an error with the Device Address API. Please try again.',
      GOODBYE: 'Bye! Thanks for using the Sample Device Address API Skill!',
      UNHANDLED: 'This skill doesn\'t support that. Please ask something else.',
      HELP: 'You can use this skill by asking something like: whats my address?',
      STOP: 'Bye! Thanks for using the Sample Device Address API Skill!',
    };
     console.log('=> Get Email -> handle -> handlerInput', JSON.stringify(handlerInput));
    const accessToken = handlerInput.requestEnvelope.context.System.apiAccessToken;
    const accessEndpoint = handlerInput.requestEnvelope.context.System.apiEndpoint;
    
    const options = {
      hostname: accessEndpoint.substring(8,accessEndpoint.length),
      path: "/v2/accounts/~current/settings/Profile.email",
      //method: "GET",
      headers: {
            "content-type": "application/json",
            "accept": "application/json",
            "authorization": "Bearer" + " " + accessToken
      }
    };
    var promise= new Promise((resolve, reject) => {
        var req = https.get(options, function(res) {
            // reject on bad status
            if (res.statusCode < 200 || res.statusCode >= 300) {
                return reject(new Error('statusCode=' + res.statusCode));
            }
            // cumulate data
            var body = [];
            res.on('data', function(chunk) {
                body.push(chunk);
            });
            // resolve on end
            res.on('end', function() {
                try {
                    body=String(body);      
                } catch(e) {
                    reject(e);
                }
                resolve(body);
            });
        });
        // reject on request error
        req.on('error', function(err) {
            // This is not a "Second reject", just a different sort of failure
            reject(err);
        });
        req.on('data', data=> {
          resolve();
        });
        req.end();
      });
    
    return promise.then( (data) => {
           console.log("process then " , data);
           return  handlerInput.responseBuilder
                          .speak(data)
                          .reprompt("reprompt message")
                          .getResponse();
    }).catch(function(err) {
      console.log("error catch: ", err);
      const PERMISSIONS = ['alexa::profile:email:read'];
      return handlerInput.responseBuilder
        .speak(messages.NOTIFY_MISSING_PERMISSIONS)
        .withAskForPermissionsConsentCard(PERMISSIONS)
        .getResponse();
    });
  },
};

const RestartIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'RestartIntent';
  },
  handle(handlerInput) {

    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "restart" ,"Param" : "'+uid+reqId+'" }';
    
    var promise= new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });    
        
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};

const ResizeIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ResizeIntent';
  },
  handle(handlerInput) {
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    const size='&size='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.integer.resolutions.resolutionsPerAuthority[0].values[0].value.id);
  //  const size2='&size='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.object.resolutions.resolutionsPerAuthority[0].values[0].value.id);
    console.log("size: "+ size);
  //    console.log("size2: "+ size2);
    invokeParams.Payload = '{ "Path" : "resize" ,"Param" : "'+uid+reqId+size+'" }';
    
    var promise= new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });    
        
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};


const ListIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ListIntent';
  },
  handle(handlerInput) {
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    const act='&action='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.actionString.resolutions.resolutionsPerAuthority[0].values[0].value.name);

   invokeParams.Payload = '{ "Path" : "list" ,"Param" : "'+uid+reqId+act+'" }';
    
    var promise= new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });    
        
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};

const ScaleIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ScaleIntent';
  },
  handle(handlerInput) {
    //TODO is querystring necessary here as I believe AWS may escape it.
   // const srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.resolutions.resolutionsPerAuthority[0].values[0].value.name);
   
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    const frac='&frac='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.fraction.resolutions.resolutionsPerAuthority[0].values[0].value.id);
    //                  handlerInput.requestEnvelope.request.intent.slots.YesNo.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "scale" ,"Param" : "'+uid+reqId+frac+'" }';
    
    var promise= new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });    
        
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};


const BookIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'BookIntent';
  },
  handle(handlerInput) {
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    const bkid='&bkid='+handlerInput.requestEnvelope.request.intent.slots.BookName.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "book" ,"Param" : "'+uid+reqId+bkid+'" }';
    
    var promise= new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};


const CloseBookIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'CloseBookIntent';
  },
  handle(handlerInput) {
    var uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "close" ,"Param" : "'+uid+reqId+'" }';

    var promise= new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};


const SearchIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'SearchIntent';
  },
  handle(handlerInput) {
    //let srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.value);
     
    //if (srch === "&srch=undefined") {
    const  srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.opentext.value);
    //}
    console.log("srch=[" + srch + "]");
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "search" ,"Param" : "'+uid+reqId+srch+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};

const ResumeIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ResumeIntent';
  },
  handle(handlerInput) {
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "resume" ,"Param" : "'+uid+reqId+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};
const BackIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'backIntent';
  },
  handle(handlerInput) {
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId   ; 
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "back" ,"Param" : "'+uid+reqId+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  
  },
};


const SelectIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'SelectIntent';
  },
  handle(handlerInput) {
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId   ; 
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    const selid='&sId='+handlerInput.requestEnvelope.request.intent.slots.integerValue.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "select" ,"Param" : "'+uid+selid+reqId+'" }';
    console.log("uid "+ uid);
    console.log("selId " + selid);
    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  
  },
};

const GotoIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'GotoIntent';
  },

  handle(handlerInput) {
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    const goId='&goId='+handlerInput.requestEnvelope.request.intent.slots.gotoId.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "goto" ,"Param" : "'+uid+reqId+goId+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  
  },
};



const NextIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'NextIntent';
  },
  handle(handlerInput) {
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "next" ,"Param" : "'+uid+reqId+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};


const RepeatIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'RepeatIntent';
  },
  handle(handlerInput) {
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "repeat" ,"Param" : "'+uid+reqId+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  
  },
};

const PrevIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'PrevIntent';
  },
  handle(handlerInput) {
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "prev" ,"Param" : "'+uid+reqId+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};

const RecipeIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'RecipeIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
    const querystring = require("querystring");
    //const bkIdRId = handlerInput.requestEnvelope.request.intent.slots.recipe.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    const rname = querystring.escape(handlerInput.requestEnvelope.request.intent.slots.recipe.resolutions.resolutionsPerAuthority[0].values[0].value.name);
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    //var path_='&rcp='+bkIdRId
    //if ( bkIdRId.length == 0 ) {
    const path_='&rcp='+rname;
    //}
    invokeParams.Payload = '{ "Path" : "recipe" ,"Param" : "'+uid+reqId+path_+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};

const VersionIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'VersionIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    const ver='&ver='+handlerInput.requestEnvelope.request.intent.slots.version.resolutions.resolutionsPerAuthority[0].values[0].value.name;
    invokeParams.Payload = '{ "Path" : "version" ,"Param" : "'+uid+reqId+ver+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};




const YesNoIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'YesNoIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
    var uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    var yesno='&yn='+handlerInput.requestEnvelope.request.intent.slots.YesNo.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "yesno" ,"Param" : "'+uid+reqId+yesno+'" }';
 
    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};


const TaskIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'TaskIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
    var uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "task" ,"Param" : "'+uid+reqId+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};

const ContainerIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ContainerIntent';
  },
  handle(handlerInput) {
        var speechText;
    var displayText;
    var recipe ;
    var uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const reqId="&reqId="+querystring.escape(handlerInput.requestEnvelope.request.requestId);
    invokeParams.Payload = '{ "Path" : "container" ,"Param" : "'+uid+reqId+'" }';

    var promise= new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};


const HelpIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'AMAZON.HelpIntent';
  },
  handle(handlerInput) {
    const speechText = 'You can say hello to me!';

    return handlerInput.responseBuilder
      .speak(speechText)
      .reprompt(speechText)
      .withSimpleCard('Hello World', speechText)
      .getResponse();
  },
};

const CancelAndStopIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && (handlerInput.requestEnvelope.request.intent.name === 'AMAZON.CancelIntent'
        || handlerInput.requestEnvelope.request.intent.name === 'AMAZON.StopIntent');
  },
  handle(handlerInput) {
    const speechText = 'Goodbye!';

    return handlerInput.responseBuilder
      .speak(speechText)
      .withSimpleCard('Hello World', speechText)
      .getResponse();
  },
};

const SessionEndedRequestHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'SessionEndedRequest';
  },
  handle(handlerInput) {
    console.log(`Session ended with reason: ${handlerInput.requestEnvelope.request.reason}`);

    return handlerInput.responseBuilder.getResponse();
  },
};

const ErrorHandler = {
  canHandle() {
    return true;
  },
  handle(handlerInput, error) {
    console.log(`Error handled: ${error.message}`);

    return handlerInput.responseBuilder
      .speak('Sorry, I can\'t understand the command. Please say again.')
      .reprompt('Sorry, I can\'t understand the command. Please say again.')
      .getResponse();
  },
};

function handleResponse (handlerInput , resp) {
        if (resp.Type === "Ingredient" ) {
          const ingrd = require('APL/' + resp.Type + '.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Text)
                            .reprompt(resp.Text)
                            .addDirective(ingrd(resp.BackBtn, resp.Header,resp.SubHdr, resp.List, " ", resp.Hint))
                            .addDirective(speakcmd())
                            .getResponse();
        } else if ( resp.Type === "IngredientErr") {
          const ingrd = require('APL/' + resp.Type + '.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Error)
                            .reprompt(resp.Text)
                            .addDirective(ingrd(resp.BackBtn, resp.Header,resp.SubHdr, resp.List," ", resp.Hint, resp.Error))
                            .addDirective(speakcmd())
                            .getResponse();
        } else if (resp.Type === "Start" || resp.Type === "StartErr") {
          const start = require('APL/' + resp.Type + '.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                          .speak(resp.Verbal)
                          .reprompt(resp.Text)
                          .addDirective(start(resp.BackBtn, resp.Header, resp.SubHdr, resp.Text, resp.List, " ", resp.Hint, resp.Error))
                          .addDirective(speakcmd())
                          .getResponse();
        } else if (resp.Type === "PartList") {
          const search = require('APL/' + resp.Type + '.js');
          return  handlerInput.responseBuilder
                          .speak(resp.Verbal)
                          .reprompt(resp.Text)
                          .addDirective(search(resp.BackBtn, resp.Header, resp.SubHdr, resp.Height, resp.List, resp.Hint, resp.Error))
                          .getResponse();
       } else if (resp.Type ==="Tripple" ) { 
         console.log("T: ", resp.Text);
          console.log("V: ",resp.Verbal);
          const tripple = require('APL/'+resp.Type+'.js'); 
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                            .reprompt(resp.Verbal)
                            .addDirective(tripple(resp.Header,resp.SubHdr, resp.ListA, resp.ListB, resp.ListC, resp.Verbal, resp.Text, resp.Hint, resp.Height))
                            .addDirective(speakcmd())
                            .getResponse();  
       } else if (resp.Type === "TrippleErr") { 
          const tripple = require('APL/'+resp.Type+'.js'); 
          return  handlerInput.responseBuilder
                            .speak(resp.Error) 
                            .reprompt(resp.Error)
                            .addDirective(tripple(resp.Header,resp.SubHdr, resp.ListA, resp.ListB, resp.ListC, resp.Verbal, resp.Text, resp.Hint, resp.Height, resp.Error ))
                            .getResponse(); 
       } else if (resp.Type === "threadedBottom" ) { 
          const tripple = require('APL/'+resp.Type+'.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                            .reprompt(resp.Verbal)
                            .addDirective(tripple(resp.Header,resp.SubHdr, resp.ListA, resp.ListB, resp.ListC, resp.Verbal, resp.Text, resp.ListD, resp.ListE, resp.ListF, resp.Thread1, resp.Thread2, resp.Color1, resp.Color2))
                            .addDirective(speakcmd())
                            .getResponse(); 
        } else if (resp.Type === "threadedTop" ) { 
          const tripple = require('APL/'+resp.Type+'.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                            .reprompt(resp.Verbal)
                            .addDirective(tripple(resp.Header,resp.SubHdr, resp.ListA, resp.ListB, resp.ListC, resp.Verbal, resp.Text, resp.ListD, resp.ListE, resp.ListF, resp.Thread1, resp.Thread2, resp.Color1, resp.Color2))
                            .addDirective(speakcmd())
                            .getResponse(); 
        } else if (resp.Type === "Select"){
          const select = require('APL/select.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(select(resp.BackBtn, resp.Header,resp.SubHdr, resp.List))
                            .getResponse();     
        } else if (resp.Type === "Search" || resp.Type === "SearchErr") {
          const search = require('APL/' + resp.Type + '.js');
           const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                          .speak(resp.Verbal)
                          .reprompt(resp.Text)
                          .addDirective(search(resp.BackBtn, resp.Header, resp.SubHdr, resp.List, " ", resp.Hint, resp.Error))
                          .addDirective(speakcmd())
                          .getResponse();
        }  else if (resp.Type === "error") {
          console.log("about to show error screen.....")
          const search = require('APL/' + resp.Type + '.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                          .speak(resp.Verbal)
                          .reprompt(resp.Text)
                          .addDirective(search(resp.Header, resp.Text, resp.Error ))
                          .addDirective(speakcmd())
                          .getResponse();
        } else {
           const search = require('APL/Search.js');
           return  handlerInput.responseBuilder
                          .speak(resp.Verbal)
                          .reprompt(resp.Text)
                          .addDirective(search(resp.Header, resp.SubHdr, resp.List))
                          .getResponse(); 
        }
}


const skillBuilder = Alexa.SkillBuilders.custom();

exports.handler = skillBuilder
  .addRequestHandlers(
    LaunchRequestHandler,
    ResumeIntentHandler,
    GetAddressIntentHandler,
    GetEmailIntentHandler,
    BookIntentHandler,
    BackIntentHandler,
    CloseBookIntentHandler,
    RestartIntentHandler,
    ResizeIntentHandler,
    RecipeIntentHandler,
    VersionIntentHandler,
    TaskIntentHandler,
    NextIntentHandler,
    YesNoIntentHandler,
    GotoIntentHandler,
    SelectIntentHandler,
    ScaleIntentHandler,
    ListIntentHandler,
    SearchIntentHandler,
    PrevIntentHandler,
    RepeatIntentHandler,
    ContainerIntentHandler,
    HelpIntentHandler,
    CancelAndStopIntentHandler,
    TestIntentHandler,
    SessionEndedRequestHandler,
    EventHandler
  )
  .addErrorHandlers(ErrorHandler)
  .withApiClient(new Alexa.DefaultApiClient())
  .lambda();