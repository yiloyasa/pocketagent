import React from "react";
import {Composition} from "remotion";
import {PocketAgentShowcase} from "./PocketAgentShowcase";

export const Root: React.FC = () => {
  return (
    <>
      <Composition
        id="PocketAgentShowcase"
        component={PocketAgentShowcase}
        width={1080}
        height={1920}
        fps={30}
        durationInFrames={1350}
        defaultProps={{useLivePhoneCapture: false}}
      />
    </>
  );
};
