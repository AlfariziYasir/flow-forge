import { useEffect, useState } from 'react';

export interface SSEEvent {
  execution_id: string;
  step_id?: string;
  status: string;
  output?: any;
  error?: string;
}

export const useSSE = (url: string) => {
  const [data, setData] = useState<SSEEvent | null>(null);

  useEffect(() => {
    const eventSource = new EventSource(url);

    eventSource.onmessage = (event) => {
      try {
        const parsedData = JSON.parse(event.data);
        setData(parsedData);
      } catch (err) {
        console.error('Failed to parse SSE data', err);
      }
    };

    eventSource.onerror = (err) => {
      console.error('EventSource failed:', err);
      eventSource.close();
    };

    return () => {
      eventSource.close();
    };
  }, [url]);

  return data;
};
