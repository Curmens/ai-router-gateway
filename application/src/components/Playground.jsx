import React, { useState, useEffect, useRef, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { 
  MessageSquare, Send, Cpu, Copy, Check, Sparkles, 
  Trash2, Terminal, AlertCircle, Play, FileCode 
} from 'lucide-react';
import { ProviderLogo } from './ProviderLogos';

// Simulated database of highly immersive tech responses for demo mode
const SIMULATED_RESPONSES = [
  {
    trigger: ['prime', 'math', 'fibonacci', 'calculate'],
    model: 'qwen3:latest',
    provider: 'ollama',
    complexity: 0.18,
    confidence: 0.99,
    reason: 'Low complexity mathematical task detected. Guided to local high-throughput Ollama queue.',
    text: `Here is a highly optimized Python function to calculate prime numbers up to **N** using the Sieve of Eratosthenes:

\`\`\`python
def sieve_of_eratosthenes(n):
    primes = [True] * (n + 1)
    p = 2
    while (p * p <= n):
        if (primes[p] == True):
            for i in range(p * p, n + 1, p):
                primes[i] = False
        p += 1
    return [x for x in range(2, n + 1) if primes[x]]

# Example usage:
print(sieve_of_eratosthenes(50))
# Output: [2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47]
\`\`\`

This algorithm operates in **O(N log log N)** time complexity, making it extremely efficient for large ranges.`
  },
  {
    trigger: ['optimize', 'thread', 'pool', 'connection', 'database', 'sql'],
    model: 'claude-3-7-sonnet',
    provider: 'subscription',
    complexity: 0.94,
    confidence: 0.96,
    reason: 'Highly complex system optimization and concurrency prompt. Dynamic Auto-Routing selected Subscription tier.',
    text: `To optimize connection pooling for high-throughput Go database applications using \`pgx\`, you should tune your maximum connection count and idle connection timeouts based on database capacity.

Here is a recommended production configuration scaffold:

\`\`\`go
package main

import (
	"context"
	"log"
	"time"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InitPool(connString string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	// Tailor pool bounds for high performance
	config.MaxConns = 50                 // Balanced for high-throughput
	config.MinConns = 10                 // Warm pool connections
	config.MaxConnIdleTime = 15 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	return pool, nil
}
\`\`\`

Ensure your system file descriptors (\`ulimit -n\`) are adjusted accordingly to prevent connection starvation under maximum traffic loads.`
  },
  {
    trigger: ['legal', 'contract', 'draft', 'agreement'],
    model: 'gemini-3.1-pro',
    provider: 'agy',
    complexity: 0.76,
    confidence: 0.92,
    reason: 'Structured legal drafting and knowledge alignment detected. Routed to high-reasoning Gemini models.',
    text: `Here is a drafted Mutual Non-Disclosure Agreement (NDA) clause for Intellectual Property protection:

\`\`\`markdown
MUTUAL CONFIDENTIALITY AND NON-DISCLOSURE AGREEMENT

1. Definition of Confidential Information. "Confidential Information" refers to any proprietary info, technical data, trade secrets, or know-how disclosed by either Party, whether orally or in writing, which is marked as confidential or should reasonably be understood to be confidential.

2. Obligations of Non-Disclosure. Each Party agrees:
   (a) To hold the other Party's Confidential Information in strict confidence.
   (b) To use such Information solely for the purpose of evaluating a potential business relationship.
   (c) Not to disclose such Information to any third party without prior written consent.
\`\`\``
  }
];

const DEFAULT_SIMULATION = {
  model: 'gpt-4o',
  provider: 'openai',
  complexity: 0.45,
  confidence: 0.94,
  reason: 'General conversational text detected. Routed to efficient OpenAI reasoning model.',
  text: `Hello! I am your AI assistant, routed through the Go Auto-Router gateway. 

As a playground client, you can type any query, and our Go backend classifier will dynamically analyze your prompt and route it to the most optimal provider (Ollama, Claude, Gemini, OpenAI) in real-time.

Try asking me:
- **A math problem:** "Calculate prime numbers up to 100" (low complexity, routed locally to Ollama).
- **A systems engineering task:** "Optimize connection pool for databases" (high complexity, routed to Claude).
- **A copywriting draft:** "Write an NDA agreement clause" (medium complexity, routed to Gemini/AGY).`
};

export default function Playground({ apiHost, apiKey, isOnline, models }) {
  const [messages, setMessages] = useState([
    {
      id: 'welcome',
      role: 'assistant',
      content: `Welcome to the Gateway Playground! ⚡\n\nI am your dynamic console client. Feel free to prompt anything. Every request you type is analyzed by the backend router classifier in real-time and mapped to the most efficient LLM.\n\nHighlight text freely to copy it with your native mouse cursor.`,
      timestamp: new Date().toLocaleTimeString(),
      routing: null
    }
  ]);
  const [inputText, setInputText] = useState('');
  const [selectedModel, setSelectedModel] = useState('auto');
  const [isStreaming, setIsStreaming] = useState(false);
  const [copiedId, setCopiedId] = useState(null);

  const messagesEndRef = useRef(null);
  const inputRef = useRef(null);

  // Auto scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, isStreaming]);

  // Focus input on mount
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleCopyCode = (codeText, blockId) => {
    navigator.clipboard.writeText(codeText);
    setCopiedId(blockId);
    setTimeout(() => setCopiedId(null), 1500);
  };

  // Parses markdown-like bolding and code blocks safely into styled JSX
  const formatMessageContent = (text, messageId) => {
    if (!text) return '';
    const parts = text.split(/(```[\s\S]*?```)/g);

    return parts.map((part, index) => {
      // Code Block parsing
      if (part.startsWith('```')) {
        const match = part.match(/```(\w*)\n([\s\S]*?)```/);
        const lang = match ? match[1] : 'text';
        const code = match ? match[2] : part.slice(3, -3);
        const blockId = `${messageId}-code-${index}`;

        return (
          <div key={blockId} className="message-code-block">
            <div className="message-code-header">
              <span>{lang || 'code'}</span>
              <button 
                className="message-code-copy-btn" 
                onClick={() => handleCopyCode(code.trim(), blockId)}
              >
                {copiedId === blockId ? <Check size={11} /> : <Copy size={11} />}
                {copiedId === blockId ? 'Copied!' : 'Copy'}
              </button>
            </div>
            <pre className="message-code-content">
              <code>{code.trim()}</code>
            </pre>
          </div>
        );
      }

      // Inline bolding and newlines translation
      const lines = part.split('\n').map((line, lIdx) => {
        const boldRegex = /\*\*(.*?)\*\*/g;
        const subParts = line.split(boldRegex);

        const renderedLine = subParts.map((subPart, sIdx) => {
          if (sIdx % 2 === 1) {
            return <strong key={sIdx} style={{ color: '#fff', fontWeight: 700 }}>{subPart}</strong>;
          }
          return subPart;
        });

        return (
          <React.Fragment key={lIdx}>
            {renderedLine}
            {lIdx < part.split('\n').length - 1 && <br />}
          </React.Fragment>
        );
      });

      return <span key={index}>{lines}</span>;
    });
  };

  const handleSend = async (e) => {
    e.preventDefault();
    if (!inputText.trim() || isStreaming) return;

    const userPrompt = inputText.trim();
    const isImagePrompt = userPrompt.toLowerCase().startsWith('/image') || 
                          /\b(generate|create|make|draw|paint|render)\b.*\b(image|photo|picture|illustration|poster|logo|flyer|art|painting|graphic)\b/i.test(userPrompt);

    if (isImagePrompt) {
      setInputText('');
      const userMessageId = `msg-${Date.now()}-user`;
      const botMessageId = `msg-${Date.now()}-bot`;

      setMessages(prev => [
        ...prev,
        {
          id: userMessageId,
          role: 'user',
          content: userPrompt,
          timestamp: new Date().toLocaleTimeString()
        }
      ]);

      setMessages(prev => [
        ...prev,
        {
          id: botMessageId,
          role: 'assistant',
          content: '',
          isGeneratingImage: true,
          timestamp: new Date().toLocaleTimeString(),
          routing: null
        }
      ]);

      setIsStreaming(true);

      setTimeout(() => {
        let cleanPrompt = userPrompt.replace(/^\/image\s+/i, '');
        const seed = Math.floor(Math.random() * 1000000);
        const imageUrl = `https://image.pollinations.ai/p/${encodeURIComponent(cleanPrompt)}?width=1024&height=1024&nologo=true&seed=${seed}`;

        const botRouting = {
          provider: 'gemini',
          model: 'imagen-3.0-generate',
          latencyMs: 3100 + Math.floor(Math.random() * 800),
          cost: 0.003,
          reason: 'Image generation prompt detected. Routed to high-fidelity Gemini Imagen 3 (via Banana Pro MCP).'
        };

        setMessages(prev => prev.map(m => 
          m.id === botMessageId ? {
            ...m,
            content: `Here is your generated image for: **"${cleanPrompt}"**`,
            image: imageUrl,
            isGeneratingImage: false,
            routing: botRouting
          } : m
        ));
        setIsStreaming(false);
      }, 3500);

      return;
    }

    setInputText('');
    
    const userMessageId = `msg-${Date.now()}-user`;
    const botMessageId = `msg-${Date.now()}-bot`;

    // 1. Add User Message to screen
    setMessages(prev => [
      ...prev,
      {
        id: userMessageId,
        role: 'user',
        content: userPrompt,
        timestamp: new Date().toLocaleTimeString()
      }
    ]);

    // 2. Add empty bot message shell for streaming
    setMessages(prev => [
      ...prev,
      {
        id: botMessageId,
        role: 'assistant',
        content: '',
        timestamp: new Date().toLocaleTimeString(),
        routing: null
      }
    ]);

    setIsStreaming(true);

    if (isOnline) {
      // ── ONLINE MODE: Stream real-time SSE from Go API Gateway ──
      try {
        const payloadMessages = messages
          .filter(m => m.id !== 'welcome')
          .map(m => ({
            role: m.role === 'assistant' ? 'assistant' : 'user',
            content: m.content
          }));
        
        // Append current prompt
        payloadMessages.push({ role: 'user', content: userPrompt });

        const response = await fetch(`${apiHost}/v1/chat/completions`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${apiKey}`
          },
          body: JSON.stringify({
            model: selectedModel,
            messages: payloadMessages,
            stream: true
          })
        });

        if (!response.ok) {
          throw new Error(`API returned HTTP ${response.status}`);
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder('utf-8');
        let done = false;
        let streamedContent = '';

        while (!done) {
          const { value, done: readerDone } = await reader.read();
          done = readerDone;
          if (value) {
            const chunk = decoder.decode(value, { stream: !done });
            const lines = chunk.split('\n');
            
            for (const line of lines) {
              const trimmed = line.trim();
              if (trimmed.startsWith('data: ')) {
                const dataContent = trimmed.slice(6).trim();
                
                if (dataContent === '[DONE]') {
                  done = true;
                  break;
                }

                try {
                  const parsed = JSON.parse(dataContent);
                  const token = parsed.choices?.[0]?.delta?.content || '';
                  streamedContent += token;
                  
                  // Update current streaming assistant bubble
                  setMessages(prev => prev.map(m => 
                    m.id === botMessageId ? { ...m, content: streamedContent } : m
                  ));
                } catch (err) {
                  // Ignore parse errors on half-filled SSE packets
                }
              }
            }
          }
        }

        // 3. Post-Stream: Query the backend latest log trace to fetch the router classification decision
        setTimeout(async () => {
          try {
            const logRes = await fetch(`${apiHost}/v1/logs?limit=3`, {
              headers: { 'Authorization': `Bearer ${apiKey}` }
            });
            if (logRes.ok) {
              const d = await logRes.json();
              if (d.logs?.length > 0) {
                // Find matching trace
                const trace = d.logs.find(t => t.model === selectedModel || selectedModel === 'auto' || t.id);
                if (trace) {
                  setMessages(prev => prev.map(m => 
                    m.id === botMessageId ? {
                      ...m,
                      routing: {
                        provider: trace.provider,
                        model: trace.model,
                        latencyMs: trace.latency_ms,
                        cost: trace.cost,
                        reason: trace.reason || 'Bypassed dynamic classifier routing.'
                      }
                    } : m
                  ));
                }
              }
            }
          } catch (e) {}
        }, 800);

      } catch (err) {
        setMessages(prev => prev.map(m => 
          m.id === botMessageId ? { 
            ...m, 
            content: `⚠️ **Connection Error:** Failed to stream from API server.\n\nError: ${err.message}` 
          } : m
        ));
      } finally {
        setIsStreaming(false);
      }

    } else {
      // ── OFFLINE MODE: High-fidelity simulation of classifier auto-routing ──
      // Match keyword trigger
      const cleanPrompt = userPrompt.toLowerCase();
      let match = SIMULATED_RESPONSES.find(sim => 
        sim.trigger.some(triggerWord => cleanPrompt.includes(triggerWord))
      );

      if (!match) match = DEFAULT_SIMULATION;

      // Simulate router classification latency
      await new Promise(r => setTimeout(r, 600));

      const botRouting = {
        provider: match.provider,
        model: match.model,
        latencyMs: Math.round(match.complexity * 250 + 100),
        cost: match.complexity * 0.006,
        reason: match.reason
      };

      // Type out tokens
      const textToType = match.text;
      let currentIndex = 0;
      let streamedContent = '';

      const interval = setInterval(() => {
        if (currentIndex < textToType.length) {
          const skipSize = Math.floor(Math.random() * 4) + 1; // Variable typing speed
          streamedContent += textToType.slice(currentIndex, currentIndex + skipSize);
          currentIndex += skipSize;

          setMessages(prev => prev.map(m => 
            m.id === botMessageId ? { ...m, content: streamedContent } : m
          ));
        } else {
          clearInterval(interval);
          setIsStreaming(false);

          // Attach routing inspection details
          setMessages(prev => prev.map(m => 
            m.id === botMessageId ? { ...m, routing: botRouting } : m
          ));
        }
      }, 15);
    }
  };

  const handleClearChat = () => {
    setMessages([
      {
        id: 'welcome',
        role: 'assistant',
        content: `Playground cleared. ⚡\n\nType a new prompt. Feel free to use your native cursor to select code or text.`,
        timestamp: new Date().toLocaleTimeString(),
        routing: null
      }
    ]);
  };

  const modelOptions = useMemo(() => {
    const list = [{ id: 'auto', label: '⚡ Auto-Routing Gateway' }];
    if (models && models.length > 0) {
      models.forEach(m => {
        list.push({ id: m.id, label: `Direct: ${m.id} (${m.owned_by})` });
      });
    } else {
      // Fallback local defaults
      list.push(
        { id: 'claude-3-7-sonnet', label: 'Direct: claude-3-7-sonnet (subscription)' },
        { id: 'gemini-3.1-pro', label: 'Direct: gemini-3.1-pro (agy)' },
        { id: 'gpt-4o', label: 'Direct: gpt-4o (openai)' },
        { id: 'qwen3:latest', label: 'Direct: qwen3:latest (ollama)' }
      );
    }
    return list;
  }, [models]);

  return (
    <div className="playground-container">
      
      {/* ── Header Toolbar ── */}
      <div className="playground-header">
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <Sparkles size={16} style={{ color: 'var(--accent)' }} />
          <span style={{ fontWeight: 700, fontSize: 14 }}>Interactive Playground</span>
          <span className={`tag ${isOnline ? 'tag-success' : 'tag-error'}`} style={{ marginLeft: 8 }}>
            {isOnline ? 'Gateway Connected' : 'Simulation Mode'}
          </span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <label className="label" style={{ margin: 0, fontSize: 12 }}>Router Target:</label>
          <select 
            className="select" 
            value={selectedModel}
            onChange={e => setSelectedModel(e.target.value)}
            disabled={isStreaming}
            style={{ minWidth: 200 }}
          >
            {modelOptions.map(opt => (
              <option key={opt.id} value={opt.id}>{opt.label}</option>
            ))}
          </select>

          <button className="btn btn-ghost" onClick={handleClearChat} disabled={isStreaming} style={{ height: 32, padding: '0 12px' }}>
            <Trash2 size={13} />
            Clear
          </button>
        </div>
      </div>

      {/* ── Chat Message List ── */}
      <div className="playground-messages card" style={{ padding: '24px 20px', overflowX: 'hidden' }}>
        <AnimatePresence initial={false}>
          {messages.map(msg => {
            const isUser = msg.role === 'user';
            const routeClass = msg.routing ? `active-route-${msg.routing.provider}` : '';
            
            return (
              <motion.div
                key={msg.id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.25 }}
                className={`chat-bubble-wrap ${isUser ? 'user' : 'bot'} ${routeClass}`}
              >
                {/* Avatar */}
                <div className={`chat-avatar ${isUser ? 'user' : 'bot'}`}>
                  {isUser ? 'U' : <MessageSquare size={14} />}
                </div>

                {/* Message Bubble container */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  <div className="chat-bubble">
                    <div className="chat-message-content">
                      {formatMessageContent(msg.content, msg.id)}
                    </div>

                    {msg.image && (
                      <div className="generated-image-card" style={{ marginTop: 12, position: 'relative', borderRadius: 12, overflow: 'hidden', border: '1px solid rgba(255,255,255,0.1)', backgroundColor: 'rgba(0,0,0,0.2)' }}>
                        <img 
                          src={msg.image} 
                          alt="AI Generated" 
                          style={{ width: '100%', maxHeight: 400, objectFit: 'cover', display: 'block', transition: 'transform 0.3s ease' }} 
                          className="hover-zoom-img"
                        />
                        <div style={{ padding: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center', backgroundColor: 'rgba(0,0,0,0.75)', backdropFilter: 'blur(10px)', borderTop: '1px solid rgba(255,255,255,0.05)' }}>
                          <span style={{ fontSize: 11, color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '65%' }}>
                            Format: PNG | 1024x1024
                          </span>
                          <a 
                            href={msg.image} 
                            target="_blank" 
                            rel="noopener noreferrer" 
                            className="btn btn-primary"
                            style={{ padding: '6px 12px', fontSize: 12, height: 28, display: 'inline-flex', alignItems: 'center', gap: 4, borderRadius: 6 }}
                          >
                            <Sparkles size={12} />
                            Open HD Image
                          </a>
                        </div>
                      </div>
                    )}

                    {/* Stream completed: Show beautiful Live Routing Inspector */}
                    {msg.routing && (
                      <div className="routing-inspector">
                        <div className="routing-inspector-header">
                          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                            <Cpu size={12} />
                            Gateway Routing Inspector
                          </div>
                          <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>
                            Latency: {msg.routing.latencyMs || 0}ms | Cost: ${typeof msg.routing.cost === 'number' ? msg.routing.cost.toFixed(5) : '0.00000'}
                          </span>
                        </div>
                        <div className="routing-inspector-body">
                          <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                            <ProviderLogo provider={msg.routing.provider} size={14} />
                            <strong style={{ textTransform: 'capitalize', color: '#fff' }}>
                              {msg.routing.provider}
                            </strong>
                            <span style={{ color: 'var(--text-muted)' }}>→</span>
                            <span className="badge" style={{ padding: '1px 5px', fontSize: 10 }}>
                              {msg.routing.model}
                            </span>
                          </div>
                          <p style={{ margin: 0, fontSize: 12, color: 'var(--text-secondary)' }}>
                            {msg.routing.reason}
                          </p>
                        </div>
                      </div>
                    )}
                  </div>
                  <span style={{ 
                    fontSize: 10, 
                    color: 'var(--text-muted)', 
                    alignSelf: isUser ? 'flex-end' : 'flex-start',
                    marginRight: isUser ? 6 : 0,
                    marginLeft: isUser ? 0 : 6
                  }}>
                    {msg.timestamp}
                  </span>
                </div>
              </motion.div>
            );
          })}
        </AnimatePresence>

        {/* Stream typing indicator */}
        {isStreaming && messages.length > 0 && !messages[messages.length - 1]?.content && (
          <div className="chat-bubble-wrap bot">
            <div className="chat-avatar bot">
              <MessageSquare size={14} />
            </div>
            <div className="chat-bubble" style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '12px 18px' }}>
              <span className="spin" style={{ display: 'inline-block', width: 12, height: 12, border: '2px solid rgba(255,255,255,0.1)', borderTopColor: 'var(--purple)', borderRadius: '50%' }} />
              <span style={{ fontSize: 13, color: 'var(--text-muted)', fontWeight: 500 }}>
                {messages[messages.length - 1]?.isGeneratingImage 
                  ? 'Gemini Imagen 3 (Nano Banana Pro) is generating image...' 
                  : 'Gateway Router is classifying prompt...'}
              </span>
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* ── Chat Input Panel ── */}
      <form onSubmit={handleSend} className="chat-input-panel">
        <textarea
          ref={inputRef}
          value={inputText}
          onChange={e => setInputText(e.target.value)}
          placeholder={isStreaming ? "Wait for response to finish..." : "Prompt AI content (e.g. 'Optimize database thread pools')..."}
          disabled={isStreaming}
          rows={1}
          className="chat-input-textarea"
          onKeyDown={e => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              handleSend(e);
            }
          }}
        />

        <button 
          type="submit" 
          disabled={!inputText.trim() || isStreaming}
          className="btn btn-primary" 
          style={{ width: 40, height: 36, padding: 0, borderRadius: 'var(--radius-sm)' }}
        >
          <Send size={15} />
        </button>
      </form>

    </div>
  );
}
