import { ApplicationUpdateStatus } from "./ApplicationUpdate";
import { useEffect } from "react";
import { ArrowUpRight, BookOpen, Github, MessagesSquare } from "lucide-react";
import type { PageProps } from "../../shared/types";
import { Button, Loading } from "../../ui";
import { PixelMark } from "../../PixelScene";
import { issueReportURL, repositoryURL, useBuildInfo } from "../../shared/buildInfo";

const resources = [
  { label: "Documentation", description: "Setup, guides and everyday workflows", href: `${repositoryURL}/blob/main/docs/README.md`, icon: BookOpen },
  { label: "GitHub repository", description: "Source code and project development", href: repositoryURL, icon: Github },
  { label: "Issue tracker", description: "Known issues and planned improvements", href: `${repositoryURL}/issues`, icon: MessagesSquare },
];

export function AboutSettings({ notify }: { notify: PageProps["notify"] }) {
  const { info, error, retry } = useBuildInfo();
  useEffect(() => {
    if (error) notify(error, true);
  }, [error]);

  return (
    <section className="panel about-settings">
      <header className="about-identity">
        <span className="about-mark"><PixelMark /></span>
        <div>
          <h2>About Sente</h2>
          <p className="muted">Self-hosted finance and budgeting for your household.</p>
        </div>
      </header>
      <div className="about-grid">
        <section className="about-resources" aria-labelledby="about-help-title">
          <h3 id="about-help-title">Help & project</h3>
          <nav aria-label="Sente resources" className="about-links">
            {resources.map(({ label, description, href, icon: Icon }) => (
              <a key={label} href={href} target="_blank" rel="noopener noreferrer" aria-label={label}>
                <Icon size={20} aria-hidden="true" />
                <span><strong>{label}</strong><small>{description}</small></span>
                <ArrowUpRight size={16} aria-hidden="true" />
              </a>
            ))}
          </nav>
          <div className="about-report">
            <a className="button primary" href={issueReportURL(info)} target="_blank" rel="noopener noreferrer">
              Report an issue <ArrowUpRight size={16} aria-hidden="true" />
            </a>
            <p className="footnote">Only build details are prefilled. Review your report before submitting it on GitHub.</p>
          </div>
        </section>
        <section className="about-installation" aria-labelledby="about-build-title">
          <h3 id="about-build-title">This installation</h3>
          {info ? (
            <>
              <div className="about-build-heading">
                <strong>{info.version === "dev" ? "Development build" : info.version}</strong>
                {info.modified && <span className="badge">Local changes</span>}
              </div>
              <dl className="about-build">
                <dt>Version</dt><dd>{info.version}</dd>
                <dt>Commit</dt><dd className="about-revision">{info.commit || "Unavailable"}</dd>
                <dt>Revision date</dt><dd>{info.built_at || "Unavailable"}</dd>
              </dl>
            </>
          ) : error ? (
            <div className="about-build-error">
              <p role="status">{error}</p>
              <Button variant="secondary" onClick={retry}>Retry</Button>
            </div>
          ) : <Loading>Loading build information</Loading>}
          <ApplicationUpdateStatus />
          <p className="footnote about-licence">Licence: <a href={`${repositoryURL}/blob/${info?.commit || "main"}/LICENSE`} target="_blank" rel="noopener noreferrer">GNU GPL v3</a>. No warranty. <a href={`${repositoryURL}/tree/${info?.commit || "main"}`} target="_blank" rel="noopener noreferrer">Source code</a>.</p>
        </section>
      </div>
    </section>
  );
}
