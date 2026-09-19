import { useState } from "react";
import { Button, Input, Card, Typography } from "antd";
import { Greet } from "../wailsjs/go/main/App";

function App() {
  const [resultText, setResultText] = useState("Reclip base OK — Tailwind + AntD + Wails binding");
  const [name, setName] = useState("");

  function greet() {
    Greet(name).then(setResultText);
  }

  return (
    <div className="min-h-screen bg-[#1b2636] text-white flex items-center justify-center p-6">
      <Card className="w-full max-w-lg" title="Reclip — base verify">
        <Typography.Paragraph>{resultText}</Typography.Paragraph>
        <div className="flex gap-2">
          <Input placeholder="Enter name" value={name} onChange={(e) => setName(e.target.value)} />
          <Button type="primary" onClick={greet}>
            Greet
          </Button>
        </div>
        <Typography.Text type="secondary">
          Tailwind layout + AntD components + Go binding working.
        </Typography.Text>
      </Card>
    </div>
  );
}

export default App;
