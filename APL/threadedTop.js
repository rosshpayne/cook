

module.exports = (header, subhdr, dataA, dataB, dataC, verbal, text, dataD, dataE, dataF, thread1, thread2, color1, color2) => { return {
    type: 'Alexa.Presentation.APL.RenderDocument',
    token: 'cook-tripple-screen',
    document: {
      type: 'APL',
      version: '1.0',
      import: [
        {
          name: 'alexa-styles',
          version: '1.0.0'
        },
        {
          name: 'alexa-layouts',
          version: '1.0.0'
        }
      ],
      resources:  [],
      styles: {
        textStylePressable: {
          values: [
            { backgroundColor: "blue",
              borderColor: "yellow",
              color: "yellow"
            }
        ]
      }
      },
      mainTemplate: {
        parameters: ['payload'],
        items: {
          when: "${@viewportProfile != @hubRoundSmall}",
          type: "Container",
          height: "100vh",
          width: "100vw",
          direction: "column",
          items: [
              {
              type: "AlexaHeader",
              headerTitle: header,
              headerSubtitle: subhdr,
              headerBackgroundColor: "rgba(240,24,25,80%)",
              headerBackButton: true,
              headerNavigationAction: "backButton"
              },
              {
              type: "Sequence",
              scrollDirection: "vertical",
              numbered: true,
              grow: 1,
              shrink: 1,
              width: "100vw",
              height: "100vh",
              items: [  
                        {
                        type: "Container",
                        direction: "column",
                        spacing: 4,
                        height: "7vh",
                        alignItems: "left",
                        justifyContent: "end",
                        items: [
                                          {
                                          type: "Text",
                                          text: thread1,
                                          color: "rgba(125,240,25,80%)",
                                          fontSize: "20dp",
                                          style: "textStylePrimary2"
                                          }
                                          ] 
                        },
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataA}",
                        spacing: 4,
                        height: "7vh",
                        alignItems: "left",
                        justifyContent: "end",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          fontSize: "12dp",
                                          color: "rgba(240,240,240,80%)",
                                          }
                                          ] 
                        },
                        {
                        type: "Frame",
                        borderColor:  "rgba(240,240,240,60%)",
                        borderWidth: 2,
                        height: "5vh",  
                        item: {
                              type: "Container",
                              direction: "column",
                              spacing: 0,
                              alignItems: "left",
                              height: "5vh",
                              justifyContent: "center",
                              items: [
                                      {
                                      type: "Text",
                                      id: "Rinstruction",
                                      text: "  ${payload.listdata.properties.text}",
                                      speech: "${payload.listdata.properties.verbal}",
                                      color: "rgba(125,240,25,90%)",
                                      fontSize: "17dp",
                                      style: "textStylePrimary2"
                                      }
                                     ] 
                        }
                        },
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataC}",
                        spacing: 10,
                        height: "12vh",
                        alignItems: "left",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          fontSize: "17dp",
                                          color: "rgba(240,240,240,80%)",
                                          }
                                          ] 
                        },
                        {
                        type: "Container",
                        direction: "column",
                        spacing: 2,
                        height: "7vh",
                        alignItems: "left",
                        justifyContent: "end",
                        items: [
                                          {
                                          type: "Text",
                                          text: thread2,
                                          fontSize: "20dp",
                                          color:  "rgba(240,240,240,80%)",
                                          style: "textStylePrimary2"
                                          }
                                          ] 
                        },
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataD}",
                        spacing: 14,
                        height: "7vh",
                        alignItems: "left",
                        justifyContent: "end",
                        items: [    
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          color: "rgba(240,240,240,80%)",
                                          fontSize: "15dp"
                                          }
                                          ] 
                        },
                        {
                        type: "Frame",
                        borderColor:  "rgba(240,240,240,60%)",
                        borderWidth: 2,
                        height: "7vh",  
                        item: {
                              type: "Container",
                              direction: "column",
                              spacing: 4,
                              data: "${payload.listdata.properties.dataE}",
                              alignItems: "left",
                              height: "7vh",
                              justifyContent: "center",
                              items: [
                                      {
                                      type: "Text",
                                      text: "${data.Title}",
                                      color: "rgba(240,240,240,80%)",
                                      fontSize: "17dp",
                                      style: "textStylePrimary2"
                                      }
                                     ] 
                        }
                        },
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataF}",
                        spacing: 4,
                        height: "32vh",
                        alignItems: "left",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          color: "rgba(240,240,240,80%)",
                                          fontSize: "15dp"
                                          }
                                          ] 
                        }
                        ]
              },
              {
                type: "AlexaFooter",
                hintText: "hint text goes here.."
              }
          ]
      }
      }
    },
    datasources: {
      listdata :    {        
            type: "object",
            properties: { 
              dataA,
              dataB,
              dataC,
              dataD,
              dataE,
              dataF,
              thread1,
              thread2,
              verbal,
              text,
              color1,
              color2
            },
            transformers: [{
                inputPath: "verbal",
                outputPath: "verbalOut",
                transformer: "ssmlToSpeech"
                },
                {
                inputPath: "verbal",
                outputPath: "text",
                transformer: "ssmlToText"
                }
            ]
            }
    }
  };
};