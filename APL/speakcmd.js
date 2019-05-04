module.exports = () => { return {
    type: "Alexa.Presentation.APL.ExecuteCommands",
    token: "cook-tripple-screen",
    commands: [
        {
          "type": "Sequential",
          "commands": [
                   {
                    type: "SpeakItem",
                    componentId : "Rinstruction"
                    },
                    {
                    type: "Idle",
                    delay: 60000
                    },
                    {                   
                    type: "Idle",
                    delay: 60000
                    }
            ]
        }
    ]
  };
};
