import React, { useMemo, useEffect } from 'react';
import ReactFlow, { 
  Background, 
  Controls, 
  useNodesState, 
  useEdgesState,
} from 'reactflow';
import 'reactflow/dist/style.css';

interface Step {
  id: string;
  action: string;
  status?: string;
  depends_on?: string[];
}

interface Props {
  steps: Step[];
}

const DAGViewer: React.FC<Props> = ({ steps }) => {
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);

  useEffect(() => {
    const newNodes = steps.map((step, index) => ({
      id: step.id,
      data: { label: `${step.id} (${step.action})` },
      position: { x: index * 200, y: 100 },
      className: step.status === 'RUNNING' ? 'node-running' : '',
      style: {
        background: step.status === 'SUCCESS' ? 'rgba(16, 185, 129, 0.2)' : 
                   step.status === 'FAILED' ? 'rgba(244, 63, 94, 0.2)' :
                   'rgba(23, 25, 35, 0.7)',
        color: '#fff',
        border: '1px solid rgba(255, 255, 255, 0.1)',
        borderRadius: '8px',
      }
    }));

    const newEdges = steps.flatMap(step => 
      (step.depends_on || []).map(dep => ({
        id: `e-${dep}-${step.id}`,
        source: dep,
        target: step.id,
        animated: step.status === 'RUNNING',
      }))
    );

    setNodes(newNodes);
    setEdges(newEdges);
  }, [steps]);

  return (
    <div style={{ width: '100%', height: '500px' }} className="glass-card">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        fitView
      >
        <Background color="#333" gap={16} />
        <Controls />
      </ReactFlow>
    </div>
  );
};

export default DAGViewer;
