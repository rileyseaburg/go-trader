import React from 'react';
import AuditTrail from './AuditTrail.jsx';
import RawDataGrid from './RawDataGrid.jsx';

export default function DebugView(props) {
  return <section className="tier debug-tier">
    <div className="tier-heading">
      <span>Tier 3</span>
      <h2>Debug view</h2>
      <p>Forensic audit tables and raw API payloads. Open only when investigating details.</p>
    </div>
    <AuditTrail data={props.data} />
    <RawDataGrid {...props} />
  </section>;
}
