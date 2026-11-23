"""用独立 PATH 模拟学习环境，验证课程版本要求及分阶段检查。"""

import os
from pathlib import Path
import shlex
import subprocess
from tempfile import TemporaryDirectory
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "scripts/study-doctor.sh"


class StudyDoctorTests(unittest.TestCase):
    def setUp(self):
        self.directory = TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.bin = Path(self.directory.name) / "bin"
        self.bin.mkdir()
        self.calls = Path(self.directory.name) / "calls"
        self.env = dict(os.environ, PATH=str(self.bin), DOCTOR_CALLS=str(self.calls))
        self.env.pop("BASH_ENV", None)
        for name, output in {
            "go": "go version go1.26.0 linux/amd64",
            "java": 'openjdk version "21.0.1"',
            "javac": "javac 21.0.1",
            "mvn": "Apache Maven 3.9.9",
            "python3": "Python 3.9.21",
            "curl": "curl 8.0.0",
            "node": "v22.12.0",
            "npm": "10.9.0",
        }.items():
            self.stub(name, output)

    def stub(self, name, output):
        path = self.bin / name
        path.write_text("#!/bin/sh\n"
                        f"printf '%s\\n' {shlex.quote(name)} >> \"$DOCTOR_CALLS\"\n"
                        f"printf '%s\\n' {shlex.quote(output)}\n")
        path.chmod(0o755)

    def docker(self, engine_ok=True):
        path = self.bin / "docker"
        path.write_text("#!/bin/sh\n"
                        "printf '%s\\n' \"docker $*\" >> \"$DOCTOR_CALLS\"\n"
                        "if [ \"$1\" = compose ]; then\n"
                        "  printf '%s\\n' 'Docker Compose version v2.30.0'\n"
                        "  exit 0\n"
                        "fi\n"
                        f"printf '%s\\n' {shlex.quote('Server 27.0' if engine_ok else 'Cannot connect to Docker daemon')}\n"
                        f"exit {0 if engine_ok else 1}\n")
        path.chmod(0o755)

    def run_doctor(self, *args):
        return subprocess.run(["/bin/bash", str(SCRIPT), *args], cwd=self.directory.name,
                              env=self.env, capture_output=True, text=True, timeout=5)

    def test_default_basics_passes_without_docker_or_node(self):
        (self.bin / "node").unlink()
        (self.bin / "npm").unlink()
        result = self.run_doctor()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("检查阶段 basics", result.stdout)
        self.assertEqual(set(self.calls.read_text().splitlines()), {"go", "java", "javac", "mvn", "python3", "curl"})

    def test_docker_flag_checks_compose_and_engine(self):
        self.docker(engine_ok=False)
        result = self.run_doctor("--docker")
        self.assertEqual(result.returncode, 1)
        self.assertIn("Docker Compose", result.stdout)
        self.assertIn("FAIL Docker Engine", result.stderr)
        self.assertIn("docker compose version", self.calls.read_text())
        self.assertIn("docker version --format", self.calls.read_text())

    def test_web_profile_checks_its_own_dependencies(self):
        for name in ["go", "java", "javac", "mvn", "curl"]:
            (self.bin / name).unlink()
        self.stub("node", "v18.12.1")
        result = self.run_doctor("--profile", "web")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(set(self.calls.read_text().splitlines()), {"node", "npm", "python3"})

    def test_electron_requires_node_22_12(self):
        self.stub("node", "v22.11.0")
        result = self.run_doctor("--profile", "electron")
        self.assertEqual(result.returncode, 1)
        self.assertIn("需要 Node.js 22.12+", result.stderr)
        self.stub("node", "v22.12.0")
        self.assertEqual(self.run_doctor("--profile", "electron").returncode, 0)

    def test_agent_requires_go_1_26_and_accepts_flag_order(self):
        self.stub("go", "go version go1.25.0 linux/amd64")
        result = self.run_doctor("--profile", "agent")
        self.assertEqual(result.returncode, 1)
        self.assertIn("需要 Go 1.26+", result.stderr)
        self.stub("go", "go version go1.26.0 linux/amd64")
        self.docker()
        self.assertEqual(self.run_doctor("--docker", "--profile", "agent").returncode, 0)

    def test_python_must_support_learning_scripts(self):
        self.stub("python3", "Python 3.8.20")
        result = self.run_doctor()
        self.assertEqual(result.returncode, 1)
        self.assertIn("需要 Python 3.9+", result.stderr)
        self.assertIn("curl", self.calls.read_text())

    def test_java_runtime_and_compiler_are_checked_separately(self):
        self.stub("java", 'java version "17.0.1"')
        result = self.run_doctor()
        self.assertEqual(result.returncode, 1)
        self.assertIn("需要 Java 21+", result.stderr)
        self.stub("java", "openjdk 21.0.1 2023-10-17")
        self.stub("javac", "javac 17.0.1")
        result = self.run_doctor()
        self.assertEqual(result.returncode, 1)
        self.assertIn("需要 JDK 21+", result.stderr)

    def test_missing_tool_reports_failure(self):
        (self.bin / "npm").unlink()
        result = self.run_doctor("--profile", "web")
        self.assertEqual(result.returncode, 1)
        self.assertIn("FAIL npm", result.stderr)
        self.assertIn("Python", result.stdout)

    def test_java_startup_options_do_not_mask_an_old_runtime(self):
        self.stub("java", 'Picked up JAVA_TOOL_OPTIONS: -Xmx 512m\njava version "17.0.1"')
        result = self.run_doctor()
        self.assertEqual(result.returncode, 1)
        self.assertIn("需要 Java 21+", result.stderr)
        self.stub("java", 'Picked up JAVA_TOOL_OPTIONS: -Xmx 512m\nopenjdk version "21.0.1"')
        result = self.run_doctor()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('Java            openjdk version "21.0.1"', result.stdout)

    def test_help_and_invalid_arguments_do_not_run_checks(self):
        result = self.run_doctor("--help")
        self.assertEqual(result.returncode, 0)
        self.assertIn("--profile", result.stdout)
        for args in [("--profile",), ("--profile", "unknown"), ("extra",), ("--docker", "extra")]:
            with self.subTest(args=args):
                result = self.run_doctor(*args)
                self.assertEqual(result.returncode, 2)
                self.assertIn("用法", result.stderr)
        self.assertFalse(self.calls.exists())


if __name__ == "__main__":
    unittest.main()
