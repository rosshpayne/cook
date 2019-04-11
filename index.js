const Alexa = require('ask-sdk-core');
var AWS = require('aws-sdk');


//var   recipe = { Response :[] };
var   invokeParams = {
        FunctionName: 'apigw-lambda-stack-3-TestFunction-1W27R33Q8ONM2',
        Qualifier: 'dev'
};

var lambda = new AWS.Lambda();
AWS.config.region = 'us-east-1';

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
  
  async handle(handlerInput) {
    // var speechText = "Hi. To find a recipe, simply search for a keyword by saying 'search keyword or recipe name' where keyword is any ingredient or ingredients related to the recipe ";
    // var displayText =  "Hi. To find a recipe, simply search for a keyword by saying 'search keyword or recipe name' where keyword is any ingredient or ingredients related to the recipe ";

    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    //const accessToken = handlerInput.requestEnvelope.context.System.apiAccessToken;
    //const accessEndpoint = handlerInput.requestEnvelope.context.System.apiEndpoint;
    
    invokeParams.Payload = '{ "Path" : "start" ,"Param" : "'+uid+'" }';
    
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
              const email=getEmail(handlerInput);

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
                      }).catch(function(err) {
                         console.log("promise 2 error catch: ", err);
                      });
        } else {
              // respType != email  
              return handleResponse(handlerInput, resp);
        }
    }).catch(function(err) {
      console.log("error catch: ", err);
           return "xyz";
    });
            
  },
};

async function getEmail(handlerInput) {
        const PERMISSIONS = ['alexa::profile:email:read'];
        const { responseBuilder, serviceClientFactory } = handlerInput;
              try {
                const upsServiceClient = serviceClientFactory.getUpsServiceClient();  
                const email = await upsServiceClient.getProfileEmail(); 
                
                return email;
                
              } catch (error) {
                if (error.name == 'ServiceError') {
                  console.log('ERROR StatusCode:' + error.statusCode + ' ' + error.message);
                }
                return responseBuilder
                  .speak(messages.NOTIFY_MISSING_EMAIL_PERMISSIONS)
                  .withAskForPermissionsConsentCard(PERMISSIONS)
                  .getResponse();
              }
}


const EventHandler = {
  canHandle: handlerInput =>
    handlerInput.requestEnvelope.request.type === 'Alexa.Presentation.APL.UserEvent',
  
  handle: handlerInput => {
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId ;

    const args = handlerInput.requestEnvelope.request.arguments;
    const event = args[0];
    //const ordinal = args[1];
    //const data = args[2];
    const selid='&sId='+args[1];

    switch (event) {
    case 'select':
      invokeParams.Payload = '{ "Path" : "select" ,"Param" : "'+uid+selid+'" }';

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
       invokeParams.Payload = '{ "Path" : "back" ,"Param" : "'+uid+'" }';
        
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


const ScaleIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ScaleIntent';
  },
  handle(handlerInput) {
    const querystring = require('querystring');
    //TODO is querystring necessary here as I believe AWS may escape it.
   // const srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.resolutions.resolutionsPerAuthority[0].values[0].value.name);
   
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const frac='&frac='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.fraction.resolutions.resolutionsPerAuthority[0].values[0].value.id);
    //                  handlerInput.requestEnvelope.request.intent.slots.YesNo.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "scale" ,"Param" : "'+uid+frac+'" }';
    
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

const DimensionIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'DimensionIntent';
  },
  handle(handlerInput) {
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const dim='&dim='+handlerInput.requestEnvelope.request.intent.slots.size.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    //                  handlerInput.requestEnvelope.request.intent.slots.YesNo.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "dimension" ,"Param" : "'+uid+dim+'" }';
    
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
      }).catch((err) => { console.log("this is in error", err, err.stack);  } );
    
  },
};

const BookIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'BookIntent';
  },
  handle(handlerInput) {
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    const bkid='&bkid='+handlerInput.requestEnvelope.request.intent.slots.BookName.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "book" ,"Param" : "'+uid+bkid+'" }';
    
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
        if (resp.Type === "header") {
          const display = require('APL/header.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(display(resp.BackBtn, resp.Header, resp.SubHdr, resp.List))
                            .getResponse();
        } else if (resp.Type === "Select") {
          const select = require('APL/select.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(select(resp.BackBtn, resp.Header,resp.SubHdr, resp.List))
                            .getResponse(); 
        }
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
    invokeParams.Payload = '{ "Path" : "book/close" ,"Param" : "'+uid+'" }';

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
        if (resp.Type === "header") {
          const display = require('APL/header.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(display(resp.Header,resp.SubHdr,  resp.List) )
                            .getResponse(); 
        } else if (resp.Type === "Select") {
          const select = require('APL/select.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(select(resp.BackBtn, resp.Header,resp.SubHdr, resp.List))
                            .getResponse(); 
        }
        }).catch(function (err) { console.log(err, err.stack);  } );
  },
};


const SearchIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'SearchIntent';
  },
  handle(handlerInput) {
    const querystring = require('querystring');
    //TODO is querystring necessary here as I believe AWS may escape it.
   // const srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.resolutions.resolutionsPerAuthority[0].values[0].value.name);
    const srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.value);
    const uid="uid="+handlerInput.requestEnvelope.session.user.userId;
    invokeParams.Payload = '{ "Path" : "search" ,"Param" : "'+uid+srch+'" }';

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
    invokeParams.Payload = '{ "Path" : "resume" ,"Param" : "'+uid+'" }';

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
    const select = require('APL/select.js');
    const ingredient = require('APL/ingredients.js');
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId   ; 
    invokeParams.Payload = '{ "Path" : "back" ,"Param" : "'+uid+'" }';

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
        if (resp.Type === "Ingredient") {
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(ingredient(resp.Header,resp.SubHdr, resp.List))
                            .getResponse();
        } else {
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(select(resp.Header,resp.SubHdr, resp.List))
                            .getResponse();     
        }
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
    const selid='&sId='+handlerInput.requestEnvelope.request.intent.slots.integerValue.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "select" ,"Param" : "'+uid+selid+'" }';
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
    const goId='&goId='+handlerInput.requestEnvelope.request.intent.slots.gotoId.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "goto" ,"Param" : "'+uid+goId+'" }';

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

const TestIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'TestIntent';
  },
  handle(handlerInput) {
    const test = require('APL/test.js');
    const speakcmd = require('APL/testspeakcmd.js');

    return  handlerInput.responseBuilder
                            .reprompt("testing")
                            .addDirective(test("Header","SubHdr", resp.output))
                            .addDirective(speakcmd())
                            .getResponse(); 
  },
};


const NextIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'NextIntent';
  },
  handle(handlerInput) {
    const uid='uid='+handlerInput.requestEnvelope.session.user.userId;
    invokeParams.Payload = '{ "Path" : "next" ,"Param" : "'+uid+'" }';

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
    invokeParams.Payload = '{ "Path" : "repeat" ,"Param" : "'+uid+'" }';

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
    invokeParams.Payload = '{ "Path" : "prev" ,"Param" : "'+uid+'" }';

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
    //var path_='&rcp='+bkIdRId
    //if ( bkIdRId.length == 0 ) {
      path_='&rcp='+rname;
    //}
    invokeParams.Payload = '{ "Path" : "recipe" ,"Param" : "'+uid+path_+'" }';

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
    const ver='&ver='+handlerInput.requestEnvelope.request.intent.slots.version.resolutions.resolutionsPerAuthority[0].values[0].value.name;
    invokeParams.Payload = '{ "Path" : "version" ,"Param" : "'+uid+ver+'" }';

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
    var yesno='&yn='+handlerInput.requestEnvelope.request.intent.slots.YesNo.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "yesno" ,"Param" : "'+uid+yesno+'" }';
 
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
    invokeParams.Payload = '{ "Path" : "task" ,"Param" : "'+uid+'" }';

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
    invokeParams.Payload = '{ "Path" : "container" ,"Param" : "'+uid+'" }';

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
        if (resp.Type === "Ingredient") {
          const ingredient = require('APL/ingredients.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(ingredient(resp.Header,resp.SubHdr, resp.List))
                            .getResponse();
        } else if (resp.Type === "PartList") {
          const search = require('APL/PartList.js');
          return  handlerInput.responseBuilder
                          .speak(resp.Verbal)
                          .reprompt(resp.Text)
                          .addDirective(search(resp.BackBtn, resp.Header, resp.SubHdr, resp.Height, resp.List))
                          .getResponse();
       } else if (resp.Type.indexOf("Tripple") === 0) { //resp.Type === "Tripple") { 
          const tripple = require('APL/'+resp.Type+'.js'); //const tripple = require('APL/tripple3.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                            .reprompt(resp.Verbal)
                            .addDirective(tripple(resp.Header,resp.SubHdr, resp.ListA, resp.ListB, resp.ListC, resp.Verbal, resp.Text))
                            .addDirective(speakcmd())
                            .getResponse();  
      // } else if (resp.Type === "Tripple2") { 
      //     const tripple = require('APL/tripple2.js');
      //     const speakcmd = require('APL/speakcmd.js');
      //     return  handlerInput.responseBuilder
      //                       .speak("here in tripple")
      //                       .reprompt(resp.Verbal)
      //                       .addDirective(tripple(resp.Header,resp.SubHdr, resp.ListA, resp.ListB, resp.ListC, resp.Verbal, resp.Text))
      //                       .addDirective(speakcmd())
      //                       .getResponse(); 
       } else if (resp.Type.indexOf("threaded") === 0 ) { 
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
        } else if (resp.Type === "Search") {
          const search = require('APL/search.js');
          return  handlerInput.responseBuilder
                          .speak(resp.Verbal)
                          .reprompt(resp.Text)
                          .addDirective(search(resp.BackBtn, resp.Header, resp.SubHdr, resp.List))
                          .getResponse();
        } else {
           const search = require('APL/search.js');
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
    TestIntentHandler,
    CloseBookIntentHandler,
    RecipeIntentHandler,
    VersionIntentHandler,
    TaskIntentHandler,
    NextIntentHandler,
    YesNoIntentHandler,
    GotoIntentHandler,
    SelectIntentHandler,
    DimensionIntentHandler,
    ScaleIntentHandler,
    SearchIntentHandler,
    PrevIntentHandler,
    RepeatIntentHandler,
    ContainerIntentHandler,
    HelpIntentHandler,
    CancelAndStopIntentHandler,
    SessionEndedRequestHandler,
    EventHandler
  )
  .addErrorHandlers(ErrorHandler)
  .withApiClient(new Alexa.DefaultApiClient())
  .lambda();