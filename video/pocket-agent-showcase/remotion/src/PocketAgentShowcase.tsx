import React from "react";
import {Video} from "@remotion/media";
import {
  AbsoluteFill,
  Easing,
  Img,
  interpolate,
  staticFile,
  useCurrentFrame,
} from "remotion";
import {EventTrace} from "./components/EventTrace";
import {FeaturePillars} from "./components/FeaturePillars";
import {PhoneFrame} from "./components/PhoneFrame";
import {STORYBOARD, scene} from "./storyboard";
import {COLORS, FONTS} from "./styles/colors";

export type PocketAgentShowcaseProps = {
  useLivePhoneCapture?: boolean;
};

const clamp = (value: number, input: number[]) =>
  interpolate(value, input, [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });

const sceneOpacity = (
  frame: number,
  start: number,
  end: number,
  finalScene = false,
) =>
  finalScene
    ? clamp(frame, [start - 16, start + 24])
    : interpolate(frame, [start - 16, start + 16, end - 16, end], [0, 1, 1, 0], {
        extrapolateLeft: "clamp",
        extrapolateRight: "clamp",
        easing: Easing.bezier(0.16, 1, 0.3, 1),
      });

const SceneSurface: React.FC<{
  opacity: number;
  children: React.ReactNode;
  transform?: string;
  blur?: number;
}> = ({opacity, children, transform = "none", blur = 0}) => (
  <AbsoluteFill
    style={{
      opacity,
      transform,
      filter: `blur(${blur}px)`,
      pointerEvents: "none",
    }}
  >
    {children}
  </AbsoluteFill>
);

const Label: React.FC<{
  children: React.ReactNode;
  color?: string;
  size?: number;
  style?: React.CSSProperties;
}> = ({children, color = COLORS.muted, size = 20, style}) => (
  <div
    style={{
      color,
      fontFamily: FONTS.mono,
      fontSize: size,
      lineHeight: 1.35,
      letterSpacing: "0.08em",
      ...style,
    }}
  >
    {children}
  </div>
);

const Title: React.FC<{
  children: React.ReactNode;
  size?: number;
  color?: string;
  style?: React.CSSProperties;
}> = ({children, size = 116, color = COLORS.text, style}) => (
  <div
    style={{
      color,
      fontFamily: FONTS.display,
      fontSize: size,
      fontWeight: 900,
      lineHeight: 0.95,
      letterSpacing: "-0.02em",
      ...style,
    }}
  >
    {children}
  </div>
);

const NodeCard: React.FC<{
  title: string;
  caption: string;
  color: string;
  left: number;
  top: number;
  progress: number;
}> = ({title, caption, color, left, top, progress}) => (
  <div
    style={{
      position: "absolute",
      left,
      top,
      width: 250,
      padding: "28px 26px",
      borderRadius: 28,
      backgroundColor: COLORS.card,
      border: `1px solid ${color}55`,
      boxShadow: `0 20px 70px ${color}18`,
      opacity: progress,
      transform: `translateY(${(1 - progress) * 36}px)`,
    }}
  >
    <div
      style={{
        width: 18,
        height: 18,
        borderRadius: "50%",
        backgroundColor: color,
        boxShadow: `0 0 26px ${color}`,
        marginBottom: 22,
      }}
    />
    <Title size={46}>{title}</Title>
    <Label size={17} style={{marginTop: 16}}>
      {caption}
    </Label>
  </div>
);

export const PocketAgentShowcase: React.FC<PocketAgentShowcaseProps> = ({
  useLivePhoneCapture = false,
}) => {
  const frame = useCurrentFrame();
  const signal = scene("signal");
  const control = scene("control-loop");
  const phone = scene("phone-demo");
  const pillars = scene("feature-pillars");
  const resilient = scene("resilient-local");
  const resolve = scene("resolve");

  const transition = (boundary: number) =>
    interpolate(frame, [boundary - 16, boundary + 16], [0, 1], {
      extrapolateLeft: "clamp",
      extrapolateRight: "clamp",
      easing: Easing.bezier(0.16, 1, 0.3, 1),
    });
  const finalDip = interpolate(frame, [resolve.end - 26, resolve.end], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });

  const signalIn = clamp(frame, [signal.start + 4, signal.start + 28]);
  const controlIn = clamp(frame, [control.start + 5, control.start + 32]);
  const phoneIn = clamp(frame, [phone.start + 4, phone.start + 38]);
  const pillarsIn = clamp(frame, [pillars.start + 4, pillars.start + 32]);
  const resilientIn = clamp(frame, [resilient.start + 4, resilient.start + 34]);
  const resolveIn = clamp(frame, [resolve.start + 4, resolve.start + 40]);
  const signalOpacity = sceneOpacity(frame, signal.start, signal.end);
  const controlOpacity = sceneOpacity(frame, control.start, control.end);
  const phoneOpacity = sceneOpacity(frame, phone.start, phone.end);
  const pillarsOpacity = sceneOpacity(frame, pillars.start, pillars.end);
  const resilientOpacity = sceneOpacity(frame, resilient.start, resilient.end);
  const resolveOpacity = sceneOpacity(frame, resolve.start, resolve.end, true);

  const backgroundScale = interpolate(frame, [0, STORYBOARD.durationInFrames], [1.04, 1.0], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const abstractOpacity = interpolate(
    frame,
    [0, control.start, phone.start, pillars.start, resolve.start],
    [0.18, 0.14, 0.08, 0.07, 0.11],
    {
      extrapolateLeft: "clamp",
      extrapolateRight: "clamp",
    },
  );

  return (
    <AbsoluteFill style={{backgroundColor: COLORS.black, color: COLORS.text, overflow: "hidden"}}>
      <Video
        src={staticFile("hyperframes/abstract-layer.mp4")}
        muted
        style={{
          position: "absolute",
          inset: 0,
          width: "100%",
          height: "100%",
          objectFit: "cover",
          opacity: abstractOpacity,
          transform: `scale(${backgroundScale})`,
          filter: "saturate(0.9) contrast(1.08)",
        }}
      />
      <div
        style={{
          position: "absolute",
          inset: 0,
          background:
            "radial-gradient(circle at 50% 34%, rgba(74,157,255,0.08), transparent 38%), #000000",
          opacity: 0.94,
        }}
      />

      <SceneSurface
        opacity={signalOpacity}
        transform={`translate3d(${(1 - signalIn) * -70}px, ${(1 - signalIn) * 36}px, 0) scale(${0.97 + signalIn * 0.03})`}
      >
        <div style={{position: "absolute", left: 82, top: 250}}>
          <Label color={COLORS.blue} size={18}>
            POCKETAGENT / LOCAL-FIRST CONTROL
          </Label>
          <Title size={128} style={{marginTop: 44, maxWidth: 760}}>
            Your agent.
          </Title>
          <Title size={128} color={COLORS.blue} style={{marginTop: 8, maxWidth: 760}}>
            In your pocket.
          </Title>
          <Label size={22} style={{marginTop: 46, maxWidth: 620}}>
            手机负责控制。Agent 在你的电脑上工作。
          </Label>
        </div>
        <div style={{position: "absolute", left: 70, top: 850}}>
          <EventTrace />
        </div>
        <Label
          color={COLORS.green}
          size={17}
          style={{position: "absolute", right: 90, bottom: 152}}
        >
          NO CLOUD RELAY · YOUR MACHINE
        </Label>
      </SceneSurface>

      <SceneSurface
        opacity={controlOpacity}
        transform={`translate3d(${(1 - controlIn) * 80}px, 0, 0) scale(${0.96 + controlIn * 0.04})`}
      >
        <div style={{position: "absolute", left: 78, top: 152}}>
          <Label color={COLORS.purple} size={18}>
            CONTROL LOOP / ARCHITECTURE
          </Label>
          <Title size={84} style={{marginTop: 34, maxWidth: 800}}>
            手机发起，电脑执行。
          </Title>
        </div>
        <EventTrace compact accent={COLORS.blue} />
        <NodeCard
          title="Phone"
          caption="control / approval / result"
          color={COLORS.blue}
          left={84}
          top={760}
          progress={controlIn}
        />
        <NodeCard
          title="Daemon"
          caption="stream / resume / route"
          color={COLORS.green}
          left={414}
          top={552}
          progress={Math.max(0, controlIn - 0.12)}
        />
        <NodeCard
          title="Agent"
          caption="your computer / native work"
          color={COLORS.purple}
          left={744}
          top={820}
          progress={Math.max(0, controlIn - 0.24)}
        />
        <Label color={COLORS.blue} style={{position: "absolute", left: 350, top: 820}}>
          STREAM
        </Label>
        <Label color={COLORS.green} style={{position: "absolute", left: 650, top: 734}}>
          APPROVAL
        </Label>
        <Label color={COLORS.purple} style={{position: "absolute", left: 420, top: 1100}}>
          RESULT
        </Label>
      </SceneSurface>

      <SceneSurface
        opacity={phoneOpacity}
        transform={`translate3d(0, ${(1 - phoneIn) * 120}px, 0) scale(${0.95 + phoneIn * 0.05})`}
        blur={(1 - phoneIn) * 5}
      >
        <div style={{position: "absolute", left: 78, top: 132}}>
          <Label color={COLORS.green} size={18}>
            VERIFIED UI SHOWCASE
          </Label>
          <Title size={72} style={{marginTop: 32, maxWidth: 720}}>
            从界面，看到能力。
          </Title>
          <Label size={20} style={{marginTop: 22, maxWidth: 510}}>
            已验证的 PocketAgent 界面，配合架构动效展示。
          </Label>
        </div>
        <div
          style={{
            position: "absolute",
            left: 246,
            top: 460,
            transform: `translateX(${interpolate(frame, [phone.start, phone.end], [40, -20], {
              extrapolateLeft: "clamp",
              extrapolateRight: "clamp",
            })}px)`,
          }}
        >
          <PhoneFrame
            frame={frame}
            startFrame={phone.start}
            durationInFrames={phone.end - phone.start}
            useLivePhoneCapture={useLivePhoneCapture}
          />
        </div>
        <div style={{position: "absolute", left: 74, bottom: 145}}>
          <EventTrace compact accent={COLORS.green} />
        </div>
        <Label color={COLORS.green} style={{position: "absolute", right: 86, bottom: 190}}>
          UI / ARCHITECTURE BRIDGE
        </Label>
      </SceneSurface>

      <SceneSurface
        opacity={pillarsOpacity}
        transform={`translate3d(0, ${(1 - pillarsIn) * 80}px, 0) scale(${0.97 + pillarsIn * 0.03})`}
      >
        <div style={{position: "absolute", left: 80, top: 148}}>
          <Label color={COLORS.blue} size={18}>
            FIVE CAPABILITY PILLARS
          </Label>
          <Title size={84} style={{marginTop: 30, maxWidth: 720}}>
            一台手机，五种入口。
          </Title>
          <Label size={19} style={{marginTop: 18}}>
            这是能力地图，不是性能统计。
          </Label>
        </div>
        <div style={{position: "absolute", left: 80, top: 650}}>
          <FeaturePillars
            frame={frame}
            startFrame={pillars.start + 34}
            features={STORYBOARD.features}
            values={[0.94, 0.78, 0.68, 0.58, 0.48]}
          />
        </div>
      </SceneSurface>

      <SceneSurface
        opacity={resilientOpacity}
        transform={`translate3d(${(1 - resilientIn) * -60}px, 0, 0) scale(${0.96 + resilientIn * 0.04})`}
      >
        <div style={{position: "absolute", left: 78, top: 160}}>
          <Label color={COLORS.green} size={18}>
            RESILIENT / LOCAL / CONTINUOUS
          </Label>
          <Title size={86} style={{marginTop: 30, maxWidth: 760}}>
            连接可以恢复，
          </Title>
          <Title size={86} color={COLORS.green} style={{marginTop: 8, maxWidth: 760}}>
            上下文不会丢。
          </Title>
        </div>
        <div
          style={{
            position: "absolute",
            left: 78,
            top: 720,
            width: 920,
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gap: 22,
          }}
        >
          {[
            ["adb reverse", "USB path", COLORS.blue],
            ["LAN discovery", "fallback route", COLORS.green],
            ["event replay", "resume context", COLORS.purple],
            ["approval recovery", "continue safely", COLORS.yellow],
            ["multi-agent switching", "switch agents", COLORS.blue],
          ].map(([title, caption, color], index) => {
            const itemProgress = clamp(frame, [resilient.start + 26 + index * 14, resilient.start + 52 + index * 14]);
            return (
              <div
                key={title}
                style={{
                  padding: "28px 26px",
                  minHeight: 170,
                  backgroundColor: COLORS.card,
                  borderRadius: 26,
                  border: `1px solid ${color}44`,
                  opacity: itemProgress,
                  transform: `translateY(${(1 - itemProgress) * 30}px)`,
                }}
              >
                <Label color={color} size={19}>
                  {title}
                </Label>
                <Title size={42} style={{marginTop: 24}}>
                  {caption}
                </Title>
              </div>
            );
          })}
        </div>
        <Label color={COLORS.muted} style={{position: "absolute", left: 84, bottom: 160}}>
          ZERO CLOUD SERVER · LOCAL-FIRST BY DESIGN
        </Label>
      </SceneSurface>

      <SceneSurface
        opacity={resolveOpacity}
        transform={`translate3d(0, ${(1 - resolveIn) * 60}px, 0) scale(${0.96 + resolveIn * 0.04})`}
      >
        <div style={{position: "absolute", left: 84, top: 310}}>
          <Title size={112}>Your computer.</Title>
          <Title size={112} color={COLORS.blue} style={{marginTop: 12}}>
            Your agents.
          </Title>
          <Title size={112} style={{marginTop: 12}}>
            Your pocket.
          </Title>
          <Label color={COLORS.green} size={22} style={{marginTop: 54}}>
            PocketAgent
          </Label>
        </div>
        <div
          style={{
            position: "absolute",
            right: 22,
            top: 620,
            opacity: resolveIn,
            transform: `scale(${0.58 + resolveIn * 0.06}) rotate(-4deg)`,
          }}
        >
          <PhoneFrame
            frame={frame}
            startFrame={resolve.start}
            durationInFrames={resolve.end - resolve.start}
            useLivePhoneCapture={useLivePhoneCapture}
          />
        </div>
        <Label color={COLORS.muted} size={17} style={{position: "absolute", left: 86, bottom: 150}}>
          CONTROL FROM ANYWHERE · EXECUTE ON YOUR MACHINE
        </Label>
      </SceneSurface>

      {[
        signal.end,
        control.end,
        phone.end,
        pillars.end,
        resilient.end,
      ].map((boundary) => (
        <div
          key={boundary}
          style={{
            position: "absolute",
            inset: 0,
            backgroundColor: COLORS.blue,
            opacity: (transition(boundary) * (1 - transition(boundary - 28))) * 0.18,
            mixBlendMode: "screen",
            pointerEvents: "none",
          }}
        />
      ))}
      <div
        style={{
          position: "absolute",
          inset: 0,
          backgroundColor: COLORS.black,
          opacity: finalDip,
          pointerEvents: "none",
        }}
      />
      <Img
        src={staticFile("captures/phone-demo-end.png")}
        style={{
          position: "absolute",
          width: 1,
          height: 1,
          opacity: 0,
        }}
      />
    </AbsoluteFill>
  );
};
