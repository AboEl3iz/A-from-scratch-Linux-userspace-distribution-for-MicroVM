import { Link } from 'react-router-dom'
import CodeBlock from './CodeBlock.jsx'
import Callout from './Callout.jsx'
import Mermaid from './Mermaid.jsx'
import { Icon, BrandGallery } from './Brand.jsx'
import { TopologyDiagram, DataLocDiagram, FrameDiagram, NetworkDiagram, MountLayoutDiagram } from './Diagrams.jsx'

function Heading(Tag) {
  return function H({ id, children, ...rest }) {
    return (
      <Tag id={id} className="group" {...rest}>
        {children}
        {id && <a href={`#${id}`} className="heading-anchor" aria-label="Link to this section">#</a>}
      </Tag>
    )
  }
}

function A({ href = '', ...rest }) {
  if (href.startsWith('/')) return <Link to={href} {...rest} />
  if (href.startsWith('#')) return <a href={href} {...rest} />
  return <a href={href} target="_blank" rel="noreferrer" {...rest} />
}

export const mdxComponents = {
  pre: CodeBlock,
  a: A,
  h2: Heading('h2'),
  h3: Heading('h3'),
  Callout,
  Mermaid,
  Icon,
  TopologyDiagram,
  DataLocDiagram,
  FrameDiagram,
  NetworkDiagram,
  MountLayoutDiagram,
  BrandGallery,
}
